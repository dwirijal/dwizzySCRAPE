package store

import (
	"context"
	"fmt"
)

type stubContentDB struct {
	execFn     func(ctx context.Context, query string, args ...any) error
	queryRowFn func(ctx context.Context, query string, args ...any) rowScanner
}

func (db *stubContentDB) Exec(ctx context.Context, query string, args ...any) error {
	if db.execFn == nil {
		return nil
	}
	return db.execFn(ctx, query, args...)
}

func (db *stubContentDB) QueryRow(ctx context.Context, query string, args ...any) rowScanner {
	if db.queryRowFn == nil {
		return stubRow{scanFn: func(dest ...any) error {
			return fmt.Errorf("unexpected QueryRow call: %s", query)
		}}
	}
	return db.queryRowFn(ctx, query, args...)
}

type stubRow struct {
	scanFn func(dest ...any) error
}

func (r stubRow) Scan(dest ...any) error {
	if r.scanFn == nil {
		return nil
	}
	return r.scanFn(dest...)
}
