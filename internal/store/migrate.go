package store

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/hosting-panel/panel/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(64) PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return err
	}
	entries, err := fs.ReadDir(db.Migrations, "migrations")
	if err != nil {
		return err
	}
	names := []string{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		var n int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version=$1`, name).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		body, err := db.Migrations.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}
