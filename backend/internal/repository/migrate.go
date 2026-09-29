// Package repository — migrate chạy golang-migrate lên PostgreSQL.
package repository

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/KernelStore/backend/internal/config"
	"github.com/KernelStore/backend/migrations"
)

// RunMigrations áp dụng mọi migration chưa chạy (nguồn: file SQL nhúng trong binary).
func RunMigrations(cfg *config.Config) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, cfg.DatabaseURL())
	if err != nil {
		return fmt.Errorf("migrate.New: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			slog.Info("Migrations: no new migrations to apply")
			return nil
		}
		return fmt.Errorf("migrate.Up: %w", err)
	}

	slog.Info("Migrations applied successfully")
	return nil
}
