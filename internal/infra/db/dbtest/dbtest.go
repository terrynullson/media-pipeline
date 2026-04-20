// Package dbtest provides per-test PostgreSQL isolation for repository and
// handler tests. Each call returns a *sql.DB whose connections are pinned to
// a freshly created schema; the schema is dropped on test cleanup.
//
// Tests are skipped (not failed) when TEST_DATABASE_URL is unset so a plain
// `go test ./...` on a developer machine without Postgres still passes the
// non-DB suites.
package dbtest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"media-pipeline/internal/infra/db"
	infraRuntime "media-pipeline/internal/infra/runtime"
)

// New opens a *sql.DB scoped to a freshly migrated schema. Skips the test
// when TEST_DATABASE_URL is unset.
func New(t *testing.T) *sql.DB {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping Postgres-backed test")
	}

	schema := uniqueSchemaName(t)
	scopedDSN, err := injectSearchPath(dsn, schema)
	if err != nil {
		t.Fatalf("inject search_path: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	bootstrap, err := db.Open(ctx, db.Options{DSN: dsn, MaxOpenConns: 1, MaxIdleConns: 1})
	if err != nil {
		t.Fatalf("open bootstrap db: %v", err)
	}
	if _, err := bootstrap.ExecContext(ctx, fmt.Sprintf(`CREATE SCHEMA %q`, schema)); err != nil {
		_ = bootstrap.Close()
		t.Fatalf("create schema %q: %v", schema, err)
	}
	_ = bootstrap.Close()

	sqlDB, err := db.Open(ctx, db.Options{DSN: scopedDSN, MaxOpenConns: 4, MaxIdleConns: 2})
	if err != nil {
		dropSchema(dsn, schema)
		t.Fatalf("open scoped db: %v", err)
	}

	migrationsPath, err := infraRuntime.ResolvePath("internal/infra/db/migrations")
	if err != nil {
		_ = sqlDB.Close()
		dropSchema(dsn, schema)
		t.Fatalf("resolve migrations path: %v", err)
	}
	if err := db.RunMigrations(ctx, sqlDB, migrationsPath); err != nil {
		_ = sqlDB.Close()
		dropSchema(dsn, schema)
		t.Fatalf("run migrations: %v", err)
	}

	t.Cleanup(func() {
		_ = sqlDB.Close()
		dropSchema(dsn, schema)
	})

	return sqlDB
}

func uniqueSchemaName(t *testing.T) string {
	t.Helper()
	var raw [6]byte
	if _, err := rand.Read(raw[:]); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return "test_" + hex.EncodeToString(raw[:])
}

func injectSearchPath(dsn, schema string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func dropSchema(dsn, schema string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, err := db.Open(ctx, db.Options{DSN: dsn, MaxOpenConns: 1, MaxIdleConns: 1})
	if err != nil {
		return
	}
	defer conn.Close()
	_, _ = conn.ExecContext(ctx, fmt.Sprintf(`DROP SCHEMA %q CASCADE`, schema))
}
