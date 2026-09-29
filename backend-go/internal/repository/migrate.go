// Package repository — migrate chạy golang-migrate lên PostgreSQL.
package repository

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/KernelStore/backend-go/internal/config"
)

// RunMigrations chạy tất cả migrations chưa áp dụng.
// migrationsDir là đường dẫn thư mục chứa file .sql (ví dụ "migrations").
func RunMigrations(cfg *config.Config, migrationsDir string) error {
	dbURL := cfg.DatabaseURL()
	sourceURL := "file://" + migrationsDir

	m, err := migrate.New(sourceURL, dbURL)
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
