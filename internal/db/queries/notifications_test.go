package queries_test

import (
	"context"
	"testing"
	"time"

	"github.com/evan/football-picks/internal/db/queries"
	"github.com/evan/football-picks/internal/testutil"
)

func TestGetFailedNotificationsExcludesSuccessful(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(-time.Hour))
	author := testutil.SeedUser(t, pool, "uid-notif-author", "Author", "author@test.com")
	testutil.SeedAnnouncement(t, pool, author.ID, week.ID, "Week 1 picks are open!")

	// User A: 1 failed send — should appear in retry results.
	userA := testutil.SeedUser(t, pool, "uid-notif-a", "UserA", "usera@test.com")
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO notification_log (user_id, week_id, success, error_msg)
		VALUES ($1, $2, false, 'timeout')
	`, userA.ID, week.ID); err != nil {
		t.Fatalf("insert failed log: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`UPDATE users SET notify_email = true WHERE id = $1`, userA.ID); err != nil {
		t.Fatalf("enable notifications: %v", err)
	}

	// User B: failed then successful — should NOT appear.
	userB := testutil.SeedUser(t, pool, "uid-notif-b", "UserB", "userb@test.com")
	for _, success := range []bool{false, true} {
		if _, err := pool.Exec(context.Background(), `
			INSERT INTO notification_log (user_id, week_id, success) VALUES ($1, $2, $3)
		`, userB.ID, week.ID, success); err != nil {
			t.Fatalf("insert log for B: %v", err)
		}
	}
	if _, err := pool.Exec(context.Background(),
		`UPDATE users SET notify_email = true WHERE id = $1`, userB.ID); err != nil {
		t.Fatalf("enable notifications: %v", err)
	}

	rows, err := queries.GetFailedNotifications(context.Background(), pool)
	if err != nil {
		t.Fatalf("GetFailedNotifications: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 retry candidate, got %d", len(rows))
	}
	if rows[0].UserID != userA.ID {
		t.Errorf("expected UserA in results, got %q", rows[0].DisplayName)
	}
}

func TestGetFailedNotificationsExcludesMaxAttempts(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(-time.Hour))
	author := testutil.SeedUser(t, pool, "uid-notif-author2", "Author2", "author2@test.com")
	testutil.SeedAnnouncement(t, pool, author.ID, week.ID, "Week 1 picks are open!")

	insertFailed := func(userID interface{}) {
		t.Helper()
		if _, err := pool.Exec(context.Background(), `
			INSERT INTO notification_log (user_id, week_id, success, error_msg)
			VALUES ($1, $2, false, 'timeout')
		`, userID, week.ID); err != nil {
			t.Fatalf("insert failed log: %v", err)
		}
	}
	enableNotif := func(userID interface{}) {
		t.Helper()
		if _, err := pool.Exec(context.Background(),
			`UPDATE users SET notify_email = true WHERE id = $1`, userID); err != nil {
			t.Fatalf("enable notifications: %v", err)
		}
	}

	// User A: 3 failed attempts — should NOT appear (HAVING COUNT(*) < 3).
	userA := testutil.SeedUser(t, pool, "uid-notif-max", "MaxUser", "max@test.com")
	insertFailed(userA.ID)
	insertFailed(userA.ID)
	insertFailed(userA.ID)
	enableNotif(userA.ID)

	// User B: 2 failed attempts — should appear.
	userB := testutil.SeedUser(t, pool, "uid-notif-two", "TwoUser", "two@test.com")
	insertFailed(userB.ID)
	insertFailed(userB.ID)
	enableNotif(userB.ID)

	rows, err := queries.GetFailedNotifications(context.Background(), pool)
	if err != nil {
		t.Fatalf("GetFailedNotifications: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 retry candidate, got %d", len(rows))
	}
	if rows[0].UserID != userB.ID {
		t.Errorf("expected UserB, got %q", rows[0].DisplayName)
	}
	if rows[0].Attempts != 2 {
		t.Errorf("attempts: got %d, want 2", rows[0].Attempts)
	}
}
