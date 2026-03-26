package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PgxContentDB struct {
	pool *pgxpool.Pool
}

type pgxRow struct {
	row pgx.Row
}

func NewPgxContentDB(ctx context.Context, rawURL string) (*PgxContentDB, error) {
	config, err := pgxpool.ParseConfig(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("parse postgres url: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &PgxContentDB{pool: pool}, nil
}

func (db *PgxContentDB) Close() {
	if db != nil && db.pool != nil {
		db.pool.Close()
	}
}

func (db *PgxContentDB) Exec(ctx context.Context, query string, args ...any) error {
	if db == nil || db.pool == nil {
		return fmt.Errorf("postgres pool is required")
	}
	_, err := db.pool.Exec(ctx, query, args...)
	return err
}

func (db *PgxContentDB) QueryRow(ctx context.Context, query string, args ...any) rowScanner {
	return pgxRow{row: db.pool.QueryRow(ctx, query, args...)}
}

func (r pgxRow) Scan(dest ...any) error {
	return r.row.Scan(dest...)
}

var _ contentDB = (*PgxContentDB)(nil)
