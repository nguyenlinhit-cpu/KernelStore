// Package repository quản lý kết nối PostgreSQL và các truy vấn SQL viết tay (pgx/v5).
package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KernelStore/backend-go/internal/config"
)

// DBTX là thứ chạy được truy vấn: *pgxpool.Pool hoặc pgx.Tx.
// Nhờ vậy cùng một hàm repository dùng được trong và ngoài transaction.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// NewPool tạo connection pool pgx tới PostgreSQL.
func NewPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	slog.Info("Connecting to database", "host", cfg.DBHost, "port", cfg.DBPort, "db", cfg.DBName)

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL())
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}
	// Đọc timestamptz ra UTC (mặc định pgx dùng giờ máy) — giống Npgsql trả DateTime Kind=Utc,
	// nên JSON luôn có hậu tố "Z" như bản C#.
	poolCfg.AfterConnect = func(_ context.Context, conn *pgx.Conn) error {
		conn.TypeMap().RegisterType(&pgtype.Type{
			Name:  "timestamptz",
			OID:   pgtype.TimestamptzOID,
			Codec: &pgtype.TimestamptzCodec{ScanLocation: time.UTC},
		})
		return nil
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

// InTx chạy fn trong một transaction; fn trả lỗi → rollback.
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, pool, fn)
}

// one đọc tối đa một dòng vào struct theo tag `db`; không có dòng → (nil, nil).
func one[T any](ctx context.Context, db DBTX, sql string, args ...any) (*T, error) {
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	v, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[T])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return v, err
}

// exists chạy "SELECT EXISTS(...)".
func exists(ctx context.Context, db DBTX, sql string, args ...any) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx, "SELECT EXISTS("+sql+")", args...).Scan(&ok)
	return ok, err
}
