package repositories

import (
	"database/sql"
	"testing"

	"media-pipeline/internal/infra/db/dbtest"
)

// openTestDB is the legacy helper used by the existing repository tests; it
// delegates to the shared dbtest package so we maintain a single source of
// truth for per-test schema isolation.
func openTestDB(t *testing.T) *sql.DB {
	return dbtest.New(t)
}
