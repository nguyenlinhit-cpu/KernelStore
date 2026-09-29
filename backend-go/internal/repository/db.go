// Package repository quản lý kết nối cơ sở dữ liệu PostgreSQL.
package repository

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KernelStore/backend-go/internal/config"
)

// NewPool tạo connection pool pgx tới PostgreSQL.
func NewPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	dbURL := cfg.DatabaseURL()
	slog.Info("Connecting to database", "host", cfg.DBHost, "port", cfg.DBPort, "db", cfg.DBName)

	poolCfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	slog.Info("Database connected")
	return pool, nil
}
