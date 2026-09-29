package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/KernelStore/backend/internal/models"
)

// Các hàm ở đây thay UserManager/UserStore của ASP.NET Identity.

const userColumns = `"Id", "FullName", "AvatarUrl", "CreatedAt", "IsActive", "Role",
	"UserName", "NormalizedUserName", "Email", "NormalizedEmail", "EmailConfirmed",
	"PasswordHash", "SecurityStamp", "ConcurrencyStamp", "PhoneNumber", "PhoneNumberConfirmed",
	"TwoFactorEnabled", "LockoutEnd", "LockoutEnabled", "AccessFailedCount"`

// FindUserByEmail = UserManager.FindByEmailAsync (so theo NormalizedEmail).
func FindUserByEmail(ctx context.Context, db DBTX, normalizedEmail string) (*models.ApplicationUser, error) {
	return one[models.ApplicationUser](ctx, db,
		`SELECT `+userColumns+` FROM "AspNetUsers" WHERE "NormalizedEmail" = $1 LIMIT 1`, normalizedEmail)
}

// FindUserByName = UserManager.FindByNameAsync (so theo NormalizedUserName).
func FindUserByName(ctx context.Context, db DBTX, normalizedUserName string) (*models.ApplicationUser, error) {
	return one[models.ApplicationUser](ctx, db,
		`SELECT `+userColumns+` FROM "AspNetUsers" WHERE "NormalizedUserName" = $1 LIMIT 1`, normalizedUserName)
}

// FindUserByID = UserManager.FindByIdAsync.
func FindUserByID(ctx context.Context, db DBTX, id uuid.UUID) (*models.ApplicationUser, error) {
	return one[models.ApplicationUser](ctx, db,
		`SELECT `+userColumns+` FROM "AspNetUsers" WHERE "Id" = $1`, id)
}

// InsertUser ghi toàn bộ cột của user mới.
func InsertUser(ctx context.Context, db DBTX, u *models.ApplicationUser) error {
	_, err := db.Exec(ctx, `INSERT INTO "AspNetUsers" (`+userColumns+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`,
		u.ID, u.FullName, u.AvatarUrl, u.CreatedAt, u.IsActive, u.Role,
		u.UserName, u.NormalizedUserName, u.Email, u.NormalizedEmail, u.EmailConfirmed,
		u.PasswordHash, u.SecurityStamp, u.ConcurrencyStamp, u.PhoneNumber, u.PhoneNumberConfirmed,
		u.TwoFactorEnabled, u.LockoutEnd, u.LockoutEnabled, u.AccessFailedCount)
	return err
}

// UpdatePasswordHash dùng khi băm lại mật khẩu (hash cũ V2 / ít vòng lặp).
// Identity đổi luôn SecurityStamp và ConcurrencyStamp trong trường hợp này.
func UpdatePasswordHash(ctx context.Context, db DBTX, id uuid.UUID, hash, securityStamp, concurrencyStamp string) error {
	_, err := db.Exec(ctx, `UPDATE "AspNetUsers"
		SET "PasswordHash" = $2, "SecurityStamp" = $3, "ConcurrencyStamp" = $4 WHERE "Id" = $1`,
		id, hash, securityStamp, concurrencyStamp)
	return err
}

// UpdateUserRoleColumn đổi cột AspNetUsers.Role (enum UserRole của dự án).
func UpdateUserRoleColumn(ctx context.Context, db DBTX, id uuid.UUID, role models.UserRole, concurrencyStamp string) error {
	_, err := db.Exec(ctx, `UPDATE "AspNetUsers" SET "Role" = $2, "ConcurrencyStamp" = $3 WHERE "Id" = $1`,
		id, role, concurrencyStamp)
	return err
}

// GetUserRoles = UserManager.GetRolesAsync: tên các role Identity của user.
func GetUserRoles(ctx context.Context, db DBTX, userID uuid.UUID) ([]string, error) {
	rows, err := db.Query(ctx, `SELECT r."Name" FROM "AspNetUserRoles" ur
		JOIN "AspNetRoles" r ON r."Id" = ur."RoleId" WHERE ur."UserId" = $1`, userID)
	if err != nil {
		return nil, err
	}
	roles, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if roles == nil {
		roles = []string{}
	}
	return roles, err
}

// IsInRole = UserManager.IsInRoleAsync.
func IsInRole(ctx context.Context, db DBTX, userID uuid.UUID, normalizedRole string) (bool, error) {
	return exists(ctx, db, `SELECT 1 FROM "AspNetUserRoles" ur
		JOIN "AspNetRoles" r ON r."Id" = ur."RoleId"
		WHERE ur."UserId" = $1 AND r."NormalizedName" = $2`, userID, normalizedRole)
}

// AddToRole = UserManager.AddToRoleAsync. Đã có role → trả added=false (Identity trả
// lỗi UserAlreadyInRole, không ghi gì). Role không tồn tại → lỗi (C# ném exception).
func AddToRole(ctx context.Context, db DBTX, userID uuid.UUID, normalizedRole string) (added bool, err error) {
	var roleID uuid.UUID
	if err := db.QueryRow(ctx, `SELECT "Id" FROM "AspNetRoles" WHERE "NormalizedName" = $1`, normalizedRole).Scan(&roleID); err != nil {
		return false, fmt.Errorf("role %s không tồn tại: %w", normalizedRole, err)
	}
	tag, err := db.Exec(ctx, `INSERT INTO "AspNetUserRoles" ("UserId", "RoleId") VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, userID, roleID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// RemoveFromRole = UserManager.RemoveFromRoleAsync.
func RemoveFromRole(ctx context.Context, db DBTX, userID uuid.UUID, normalizedRole string) error {
	_, err := db.Exec(ctx, `DELETE FROM "AspNetUserRoles" ur USING "AspNetRoles" r
		WHERE ur."RoleId" = r."Id" AND ur."UserId" = $1 AND r."NormalizedName" = $2`, userID, normalizedRole)
	return err
}
