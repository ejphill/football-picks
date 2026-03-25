package queries_test

import (
	"context"
	"testing"

	"github.com/evan/football-picks/internal/db/queries"
	"github.com/evan/football-picks/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestCreateUser(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	u, err := queries.CreateUser(context.Background(), pool, "uid-cu-1", "Alice", "alice@test.com")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.SupabaseUID != "uid-cu-1" {
		t.Errorf("supabase_uid: got %q, want uid-cu-1", u.SupabaseUID)
	}
	if u.DisplayName != "Alice" {
		t.Errorf("display_name: got %q, want Alice", u.DisplayName)
	}
	if u.Email != "alice@test.com" {
		t.Errorf("email: got %q, want alice@test.com", u.Email)
	}
	if u.ID == (uuid.UUID{}) {
		t.Error("ID should be set")
	}
}

func TestGetUserBySupabaseUID(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	seeded := testutil.SeedUser(t, pool, "uid-gus-1", "Bob", "bob@test.com")

	got, err := queries.GetUserBySupabaseUID(context.Background(), pool, "uid-gus-1")
	if err != nil {
		t.Fatalf("GetUserBySupabaseUID: %v", err)
	}
	if got.ID != seeded.ID {
		t.Errorf("ID mismatch: got %v, want %v", got.ID, seeded.ID)
	}
	if got.DisplayName != "Bob" {
		t.Errorf("display_name: got %q, want Bob", got.DisplayName)
	}
}

func TestGetUserBySupabaseUID_NotFound(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	_, err := queries.GetUserBySupabaseUID(context.Background(), pool, "uid-nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent user, got nil")
	}
	// Should wrap pgx.ErrNoRows.
	if !isNoRows(err) {
		t.Errorf("expected ErrNoRows, got %v", err)
	}
}

func TestUpdateUser(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	u := testutil.SeedUser(t, pool, "uid-uu-1", "Original", "orig2@test.com")

	updated, err := queries.UpdateUser(context.Background(), pool, u.ID, "Renamed", true)
	if err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if updated.DisplayName != "Renamed" {
		t.Errorf("display_name: got %q, want Renamed", updated.DisplayName)
	}
	if !updated.NotifyEmail {
		t.Error("notify_email should be true after update")
	}
	if updated.ID != u.ID {
		t.Error("ID should not change on update")
	}
}

// isNoRows checks whether err wraps pgx.ErrNoRows.
func isNoRows(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() != "" && (err == pgx.ErrNoRows || containsNoRows(err))
}

func containsNoRows(err error) bool {
	return err != nil && (len(err.Error()) > 0) && pgxErrNoRows(err)
}

func pgxErrNoRows(err error) bool {
	type unwrapper interface{ Unwrap() error }
	for err != nil {
		if err == pgx.ErrNoRows {
			return true
		}
		u, ok := err.(unwrapper)
		if !ok {
			break
		}
		err = u.Unwrap()
	}
	return false
}
