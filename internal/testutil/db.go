package testutil

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultTestDSN = "postgres://picks:picks@localhost:5432/footballpicks_test"

// NewTestDB creates a pgxpool connected to the test database.
// Set TEST_DATABASE_URL to override the default DSN.
func NewTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = defaultTestDSN
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("testutil: connect to test db: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

// ResetDB truncates all tables between tests.
func ResetDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		TRUNCATE notification_log, picks, announcements,
		         games, weeks, seasons, users RESTART IDENTITY CASCADE
	`)
	if err != nil {
		t.Fatalf("testutil: reset db: %v", err)
	}
}
