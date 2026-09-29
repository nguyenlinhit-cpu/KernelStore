package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KernelStore/backend-go/internal/config"
	"github.com/KernelStore/backend-go/internal/identity"
	"github.com/KernelStore/backend-go/internal/models"
	"github.com/KernelStore/backend-go/internal/services"
)

// seedRolesAndAdmin tạo 3 roles (Customer/Seller/Admin) và tài khoản admin@ks.com
// nếu chưa tồn tại — tương đương DatabaseSeeder.SeedAsync() trong C#.
func seedRolesAndAdmin(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config) error {
	_ = cfg // sẽ dùng nếu cần config seed

	// ── Seed roles ───────────────────────────────────────────────────────
	roleNames := []string{"Customer", "Seller", "Admin"}
	for _, name := range roleNames {
		normalized := identity.Normalize(name)
		var exists bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM "AspNetRoles" WHERE "NormalizedName" = $1)`,
			normalized,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check role %s: %w", name, err)
		}
		if exists {
			slog.Debug("Role already exists", "role", name)
			continue
		}

		roleID := services.NewUUID()
		stamp := identity.NewConcurrencyStamp()
		_, err = pool.Exec(ctx,
			`INSERT INTO "AspNetRoles" ("Id", "Name", "NormalizedName", "ConcurrencyStamp")
			 VALUES ($1, $2, $3, $4)`,
			roleID, name, normalized, stamp,
		)
		if err != nil {
			return fmt.Errorf("insert role %s: %w", name, err)
		}
		slog.Info("Seeded role", "role", name)
	}

	// ── Seed admin user ──────────────────────────────────────────────────
	adminEmail := "admin@ks.com"
	normalizedEmail := identity.Normalize(adminEmail)
	var adminExists bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM "AspNetUsers" WHERE "NormalizedEmail" = $1)`,
		normalizedEmail,
	).Scan(&adminExists)
	if err != nil {
		return fmt.Errorf("check admin: %w", err)
	}

	if adminExists {
		slog.Debug("Admin user already exists")
		return nil
	}

	adminID := services.NewUUID()
	userName := "admin"
	normalizedUserName := identity.Normalize(userName)
	stamp := identity.NewSecurityStamp()
	concurrencyStamp := identity.NewConcurrencyStamp()
	// Hash PBKDF2 đúng định dạng ASP.NET Identity V3 → tương thích dữ liệu bản C#.
	passwordHash := identity.HashPassword("Admin@12345")

	admin := &models.ApplicationUser{
		ID:                   adminID,
		FullName:             "System Admin",
		AvatarUrl:            "",
		CreatedAt:            time.Now().UTC(),
		IsActive:             true,
		Role:                 models.UserRoleAdmin,
		UserName:             &userName,
		NormalizedUserName:   &normalizedUserName,
		Email:                &adminEmail,
		NormalizedEmail:      &normalizedEmail,
		EmailConfirmed:       false,
		PasswordHash:         &passwordHash,
		SecurityStamp:        &stamp,
		ConcurrencyStamp:     &concurrencyStamp,
		PhoneNumber:          nil,
		PhoneNumberConfirmed: false,
		TwoFactorEnabled:     false,
		LockoutEnd:           nil,
		LockoutEnabled:       true,
		AccessFailedCount:    0,
	}

	// Tạo user + gán role trong 1 transaction để không bao giờ còn admin thiếu role.
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`INSERT INTO "AspNetUsers"
		 ("Id", "FullName", "AvatarUrl", "CreatedAt", "IsActive", "Role",
		  "UserName", "NormalizedUserName", "Email", "NormalizedEmail",
		  "EmailConfirmed", "PasswordHash", "SecurityStamp", "ConcurrencyStamp",
		  "PhoneNumber", "PhoneNumberConfirmed", "TwoFactorEnabled",
		  "LockoutEnd", "LockoutEnabled", "AccessFailedCount")
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`,
		admin.ID, admin.FullName, admin.AvatarUrl, admin.CreatedAt, admin.IsActive, int(admin.Role),
		admin.UserName, admin.NormalizedUserName, admin.Email, admin.NormalizedEmail,
		admin.EmailConfirmed, admin.PasswordHash, admin.SecurityStamp, admin.ConcurrencyStamp,
		admin.PhoneNumber, admin.PhoneNumberConfirmed, admin.TwoFactorEnabled,
		admin.LockoutEnd, admin.LockoutEnabled, admin.AccessFailedCount,
	)
	if err != nil {
		return fmt.Errorf("insert admin: %w", err)
	}

	// Gán role Admin cho admin user.
	var adminRoleID uuid.UUID
	err = tx.QueryRow(ctx,
		`SELECT "Id" FROM "AspNetRoles" WHERE "NormalizedName" = 'ADMIN'`,
	).Scan(&adminRoleID)
	if err != nil {
		return fmt.Errorf("find admin role: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO "AspNetUserRoles" ("UserId", "RoleId") VALUES ($1, $2)`,
		admin.ID, adminRoleID,
	)
	if err != nil {
		return fmt.Errorf("assign admin role: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit admin seed: %w", err)
	}

	slog.Info("Seeded admin user", "email", adminEmail)
	return nil
}
