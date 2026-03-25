package notify_test

import (
	"context"
	"testing"
	"time"

	"github.com/evan/football-picks/internal/notify"
	"github.com/evan/football-picks/internal/testutil"
	"github.com/google/uuid"
)

func TestRetryFailed_NoFailures(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	m := &countMailer{}
	notify.RetryFailed(context.Background(), pool, m)

	if m.calls.Load() != 0 {
		t.Errorf("expected 0 calls with no failures, got %d", m.calls.Load())
	}
}

func TestRetryFailed_RetriesFailedNotification(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
	author := testutil.SeedUser(t, pool, "uid-retry-author", "Author", "retry-author@test.com")
	u := testutil.SeedUser(t, pool, "uid-retry-1", "RetryUser", "retry@test.com")
	testutil.SeedAnnouncement(t, pool, author.ID, week.ID, "retry intro")

	// Log one failure for u.
	pool.Exec(context.Background(),
		`INSERT INTO notification_log (user_id, week_id, success, error_msg) VALUES ($1, $2, FALSE, 'timeout')`,
		u.ID, week.ID)

	m := &countMailer{}
	notify.RetryFailed(context.Background(), pool, m)

	if m.calls.Load() != 1 {
		t.Errorf("expected 1 retry call, got %d", m.calls.Load())
	}

	// A new log row should have been written for the retry.
	var count int
	pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM notification_log WHERE user_id=$1`, u.ID).Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 log rows after retry, got %d", count)
	}
}

func TestRetryFailed_SkipsAfterThreeAttempts(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
	author := testutil.SeedUser(t, pool, "uid-retry-auth2", "Author2", "retry-auth2@test.com")
	u := testutil.SeedUser(t, pool, "uid-retry-2", "MaxUser", "max@test.com")
	testutil.SeedAnnouncement(t, pool, author.ID, week.ID, "max retry intro")

	// Log 3 failures — should be excluded from retry (HAVING COUNT(*) < 3).
	for range 3 {
		pool.Exec(context.Background(),
			`INSERT INTO notification_log (user_id, week_id, success, error_msg) VALUES ($1, $2, FALSE, 'err')`,
			u.ID, week.ID)
	}

	m := &countMailer{}
	notify.RetryFailed(context.Background(), pool, m)

	if m.calls.Load() != 0 {
		t.Errorf("expected 0 calls after 3 attempts, got %d", m.calls.Load())
	}
}

func TestRetryFailed_SkipsIfAlreadySucceeded(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
	author := testutil.SeedUser(t, pool, "uid-retry-auth3", "Author3", "retry-auth3@test.com")
	u := testutil.SeedUser(t, pool, "uid-retry-3", "SucceededUser", "succeeded@test.com")
	testutil.SeedAnnouncement(t, pool, author.ID, week.ID, "skip intro")

	// Log one failure then one success — should not be retried.
	pool.Exec(context.Background(),
		`INSERT INTO notification_log (user_id, week_id, success) VALUES ($1, $2, FALSE)`,
		u.ID, week.ID)
	pool.Exec(context.Background(),
		`INSERT INTO notification_log (user_id, week_id, success) VALUES ($1, $2, TRUE)`,
		u.ID, week.ID)

	m := &countMailer{}
	notify.RetryFailed(context.Background(), pool, m)

	if m.calls.Load() != 0 {
		t.Errorf("expected 0 calls (already succeeded), got %d", m.calls.Load())
	}
}

func TestRetryFailed_CancelledContext(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
	author := testutil.SeedUser(t, pool, "uid-retry-auth4", "Author4", "retry-auth4@test.com")
	testutil.SeedAnnouncement(t, pool, author.ID, week.ID, "cancel intro")

	for range 3 {
		u := testutil.SeedUser(t, pool, uuid.New().String(), "U", uuid.New().String()+"@test.com")
		pool.Exec(context.Background(),
			`INSERT INTO notification_log (user_id, week_id, success) VALUES ($1, $2, FALSE)`,
			u.ID, week.ID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	m := &countMailer{}
	// Should not panic.
	notify.RetryFailed(ctx, pool, m)
}
