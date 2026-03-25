package notify_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/evan/football-picks/internal/models"
	"github.com/evan/football-picks/internal/notify"
	"github.com/evan/football-picks/internal/testutil"
	"github.com/google/uuid"
)

// countMailer records how many sends succeeded / failed.
type countMailer struct {
	failFor map[uuid.UUID]bool
	calls   atomic.Int64
}

func (m *countMailer) SendAnnouncement(_ context.Context, u models.User, _ *models.Announcement, _ int) error {
	m.calls.Add(1)
	if m.failFor[u.ID] {
		return errors.New("simulated send failure")
	}
	return nil
}

func TestSendAll_Empty(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	m := &countMailer{}
	a := &models.Announcement{ID: uuid.New(), Intro: "hi"}
	notify.SendAll(context.Background(), pool, m, nil, a, 1, 0)

	if m.calls.Load() != 0 {
		t.Errorf("expected 0 calls for empty user list, got %d", m.calls.Load())
	}
}

func TestSendAll_LogsSuccess(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
	u := testutil.SeedUser(t, pool, "uid-sa-1", "SendUser", "send@test.com")

	m := &countMailer{}
	a := &models.Announcement{ID: uuid.New(), WeekID: week.ID, Intro: "hello"}
	users := []models.User{*u}

	notify.SendAll(context.Background(), pool, m, users, a, week.WeekNumber, week.ID)

	if m.calls.Load() != 1 {
		t.Errorf("expected 1 send call, got %d", m.calls.Load())
	}

	// Verify notification_log row was written.
	var count int
	pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM notification_log WHERE user_id=$1 AND week_id=$2 AND success=TRUE`,
		u.ID, week.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 success log row, got %d", count)
	}
}

func TestSendAll_LogsFailure(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
	u := testutil.SeedUser(t, pool, "uid-sa-2", "FailUser", "fail@test.com")

	m := &countMailer{failFor: map[uuid.UUID]bool{u.ID: true}}
	a := &models.Announcement{ID: uuid.New(), WeekID: week.ID, Intro: "hi"}

	notify.SendAll(context.Background(), pool, m, []models.User{*u}, a, week.WeekNumber, week.ID)

	var count int
	pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM notification_log WHERE user_id=$1 AND success=FALSE`,
		u.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 failure log row, got %d", count)
	}
}

func TestSendAll_MultipleUsers(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))

	var users []models.User
	for i := range 5 {
		u := testutil.SeedUser(t, pool, uuid.New().String(), "User", "user"+string(rune('0'+i))+"@test.com")
		users = append(users, *u)
	}

	m := &countMailer{}
	a := &models.Announcement{ID: uuid.New(), WeekID: week.ID, Intro: "hi"}
	notify.SendAll(context.Background(), pool, m, users, a, week.WeekNumber, week.ID)

	if got := m.calls.Load(); got != 5 {
		t.Errorf("expected 5 send calls, got %d", got)
	}
}

func TestSendAll_CancelledContext(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))

	var users []models.User
	for range 10 {
		u := testutil.SeedUser(t, pool, uuid.New().String(), "U", uuid.New().String()+"@test.com")
		users = append(users, *u)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	m := &countMailer{}
	a := &models.Announcement{ID: uuid.New(), WeekID: week.ID, Intro: "hi"}
	// Should not panic, even with cancelled context.
	notify.SendAll(ctx, pool, m, users, a, week.WeekNumber, week.ID)
}
