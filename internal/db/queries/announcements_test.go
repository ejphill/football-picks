package queries_test

import (
	"context"
	"testing"
	"time"

	"github.com/evan/football-picks/internal/db/queries"
	"github.com/evan/football-picks/internal/testutil"
	"github.com/google/uuid"
)

func TestCreateAndGetAnnouncement(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
	author := testutil.SeedUser(t, pool, "uid-ann-1", "Author", "author-q@test.com")

	created, err := queries.CreateAnnouncement(context.Background(), pool, author.ID, week.ID, "Week 1 is here!")
	if err != nil {
		t.Fatalf("CreateAnnouncement: %v", err)
	}
	if created.Intro != "Week 1 is here!" {
		t.Errorf("intro: got %q", created.Intro)
	}
	if created.WeekID != week.ID {
		t.Errorf("week_id: got %d, want %d", created.WeekID, week.ID)
	}

	// Get by ID.
	got, err := queries.GetAnnouncementByID(context.Background(), pool, created.ID)
	if err != nil {
		t.Fatalf("GetAnnouncementByID: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID mismatch")
	}
}

func TestGetAnnouncementByID_NotFound(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	_, err := queries.GetAnnouncementByID(context.Background(), pool, uuid.New())
	if err == nil {
		t.Fatal("expected error for missing announcement")
	}
}

func TestGetAnnouncementsBySeason(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week1 := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
	week2 := testutil.SeedWeek(t, pool, season.ID, 2, time.Now().Add(2*time.Hour))
	author := testutil.SeedUser(t, pool, "uid-ann-2", "Author2", "author2-q@test.com")

	testutil.SeedAnnouncement(t, pool, author.ID, week1.ID, "Week 1")
	testutil.SeedAnnouncement(t, pool, author.ID, week2.ID, "Week 2")

	list, err := queries.GetAnnouncementsBySeason(context.Background(), pool, 2025)
	if err != nil {
		t.Fatalf("GetAnnouncementsBySeason: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 announcements, got %d", len(list))
	}
}

func TestGetAnnouncementsBySeason_Empty(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	list, err := queries.GetAnnouncementsBySeason(context.Background(), pool, 2099)
	if err != nil {
		t.Fatalf("GetAnnouncementsBySeason: %v", err)
	}
	if list != nil {
		t.Errorf("expected nil/empty for nonexistent season, got %v", list)
	}
}

func TestGetUsersForNotification(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))

	u1 := testutil.SeedUser(t, pool, "uid-notif-u1", "NotifyOn", "notifyon@test.com")
	u2 := testutil.SeedUser(t, pool, "uid-notif-u2", "NotifyOff", "notifyoff@test.com")

	// notify_email defaults to TRUE — disable it for u2 so only u1 is notifiable.
	pool.Exec(context.Background(),
		`UPDATE users SET notify_email=FALSE WHERE id=$1`, u2.ID)

	users, err := queries.GetUsersForNotification(context.Background(), pool, week.ID)
	if err != nil {
		t.Fatalf("GetUsersForNotification: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user with notifications enabled, got %d", len(users))
	}
	if users[0].ID != u1.ID {
		t.Errorf("expected user u1, got %q", users[0].DisplayName)
	}

	// After a successful notification log, u1 should be excluded from future sends.
	if err := queries.LogNotification(context.Background(), pool, u1.ID, week.ID, true, ""); err != nil {
		t.Fatalf("LogNotification: %v", err)
	}
	users, err = queries.GetUsersForNotification(context.Background(), pool, week.ID)
	if err != nil {
		t.Fatalf("GetUsersForNotification after log: %v", err)
	}
	if len(users) != 0 {
		t.Errorf("expected 0 users after successful notification, got %d", len(users))
	}
}

func TestLogNotification(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
	u := testutil.SeedUser(t, pool, "uid-log-1", "LogUser", "log@test.com")

	if err := queries.LogNotification(context.Background(), pool, u.ID, week.ID, true, ""); err != nil {
		t.Fatalf("LogNotification success: %v", err)
	}
	if err := queries.LogNotification(context.Background(), pool, u.ID, week.ID, false, "timeout"); err != nil {
		t.Fatalf("LogNotification failure: %v", err)
	}

	var count int
	pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM notification_log WHERE user_id=$1`, u.ID).Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 log rows, got %d", count)
	}
}

func TestGetAnnouncementsByWeek(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
	author := testutil.SeedUser(t, pool, "uid-ann-3", "Author3", "author3-q@test.com")

	testutil.SeedAnnouncement(t, pool, author.ID, week.ID, "Week 1 note")

	list, err := queries.GetAnnouncementsByWeek(context.Background(), pool, week.ID)
	if err != nil {
		t.Fatalf("GetAnnouncementsByWeek: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1, got %d", len(list))
	}
}
