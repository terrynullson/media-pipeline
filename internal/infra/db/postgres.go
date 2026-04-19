// Package db owns the bootstrap and migration of the application's PostgreSQL
// database. The rest of the codebase consumes the resulting *sql.DB through
// repositories under internal/infra/db/repositories.
//
// Driver choice: github.com/jackc/pgx/v5/stdlib registers a database/sql driver
// named "pgx". Using database/sql keeps clean architecture intact (repositories
// and tests never import vendor-specific types) while gaining pgx's modern,
// production-grade connection handling.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Options carries the small set of pool/connection knobs we expose. Anything
// that PostgreSQL can express through the DSN itself (host, port, user, db,
// sslmode, application_name, statement_timeout, etc.) belongs in DSN — keep
// this struct intentionally minimal.
type Options struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	PingTimeout     time.Duration
}

// Open returns a *sql.DB ready for use, with a context-bounded ping verifying
// that PostgreSQL is actually reachable. Pool defaults are conservative and
// tunable via Options; do not log the DSN — it can contain a password.
func Open(ctx context.Context, opts Options) (*sql.DB, error) {
	if opts.DSN == "" {
		return nil, fmt.Errorf("postgres DSN is empty")
	}

	sqlDB, err := sql.Open("pgx", opts.DSN)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	if opts.MaxOpenConns <= 0 {
		opts.MaxOpenConns = 25
	}
	if opts.MaxIdleConns <= 0 {
		opts.MaxIdleConns = 5
	}
	if opts.ConnMaxLifetime <= 0 {
		opts.ConnMaxLifetime = 30 * time.Minute
	}
	if opts.ConnMaxIdleTime <= 0 {
		opts.ConnMaxIdleTime = 5 * time.Minute
	}
	if opts.PingTimeout <= 0 {
		opts.PingTimeout = 5 * time.Second
	}

	sqlDB.SetMaxOpenConns(opts.MaxOpenConns)
	sqlDB.SetMaxIdleConns(opts.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(opts.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(opts.ConnMaxIdleTime)

	pingCtx, cancel := context.WithTimeout(ctx, opts.PingTimeout)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return sqlDB, nil
}

// RunMigrations applies every *.sql file in migrationsDir in lexical order
// inside its own transaction, recording the version in schema_migrations.
// Each migration is expected to be PostgreSQL-compatible.
func RunMigrations(ctx context.Context, sqlDB *sql.DB, migrationsDir string) error {
	if _, err := sqlDB.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT        PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) == ".sql" {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var exists int
		err := sqlDB.QueryRowContext(ctx,
			"SELECT 1 FROM schema_migrations WHERE version = $1", name,
		).Scan(&exists)
		if err == nil {
			continue
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("check migration %s: %w", name, err)
		}

		sqlBytes, readErr := os.ReadFile(filepath.Join(migrationsDir, name))
		if readErr != nil {
			return fmt.Errorf("read migration %s: %w", name, readErr)
		}

		tx, beginErr := sqlDB.BeginTx(ctx, nil)
		if beginErr != nil {
			return fmt.Errorf("begin migration tx: %w", beginErr)
		}

		if _, execErr := tx.ExecContext(ctx, string(sqlBytes)); execErr != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, execErr)
		}
		if _, insertErr := tx.ExecContext(ctx,
			"INSERT INTO schema_migrations(version) VALUES ($1)", name,
		); insertErr != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, insertErr)
		}
		if commitErr := tx.Commit(); commitErr != nil {
			return fmt.Errorf("commit migration %s: %w", name, commitErr)
		}
	}

	return nil
}
