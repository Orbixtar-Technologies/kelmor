package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/hosting-panel/panel/db"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}

	const advisoryLock = `hashtextextended('panel-schema-migrations', 0)`
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(`+advisoryLock+`)`); err != nil {
		conn.Release()
		return fmt.Errorf("acquire migration advisory lock: %w", err)
	}
	defer func() {
		unlockContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var unlocked bool
		if err := conn.QueryRow(unlockContext, `SELECT pg_advisory_unlock(`+advisoryLock+`)`).Scan(&unlocked); err != nil || !unlocked {
			closeContext, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = conn.Conn().Close(closeContext)
			closeCancel()
		}
		conn.Release()
	}()

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
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
		body, err := db.Migrations.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if err := applyMigration(ctx, conn, name, body); err != nil {
			return err
		}
	}
	return nil
}

func applyMigration(ctx context.Context, conn *pgxpool.Conn, name string, body []byte) error {
	const maxAttempts = 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}
		var applied bool
		err = tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM schema_migrations WHERE version=$1
			)`, name).Scan(&applied)
		if err == nil && !applied {
			_, err = tx.Exec(ctx, string(body))
		}
		if err == nil && !applied {
			_, err = tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES ($1)`, name)
		}
		if err == nil {
			err = tx.Commit(ctx)
		} else {
			_ = tx.Rollback(ctx)
		}
		if err == nil {
			return nil
		}
		if !retryableMigrationError(err) || attempt == maxAttempts {
			return fmt.Errorf("migration %s: %w", name, err)
		}
		delay := time.Duration(attempt*attempt) * 25 * time.Millisecond
		select {
		case <-ctx.Done():
			return fmt.Errorf("migration %s: %w", name, ctx.Err())
		case <-time.After(delay):
		}
	}
	return fmt.Errorf("migration %s exhausted retries", name)
}

func retryableMigrationError(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case "40001", "40P01":
		return true
	default:
		return false
	}
}
