package espn

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/evan/football-picks/internal/models"
	"github.com/evan/football-picks/internal/testutil"
)

func TestNewSyncer(t *testing.T) {
	pool := testutil.NewTestDB(t)
	s := NewSyncer(pool)
	if s == nil {
		t.Fatal("NewSyncer returned nil")
	}
	if s.pool == nil {
		t.Fatal("Syncer.pool is nil")
	}
	if s.client == nil {
		t.Fatal("Syncer.client is nil")
	}
}

func TestSetOnScored(t *testing.T) {
	pool := testutil.NewTestDB(t)
	s := NewSyncer(pool)
	called := false
	s.SetOnScored(func() { called = true })
	if s.onScored == nil {
		t.Fatal("onScored callback not set")
	}
	s.onScored()
	if !called {
		t.Error("onScored callback was not called")
	}
}

func TestSyncWeek_Success(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))

	// Build a fake ESPN response with one scheduled game.
	sb := ScoreboardResponse{
		Events: []Event{
			{
				ID:   "espn-sync-1",
				Date: "2025-09-07T17:00:00Z",
				Competitions: []Competition{
					{
						Competitors: []Competitor{
							{HomeAway: "home", Team: Team{Abbreviation: "KC", DisplayName: "Kansas City Chiefs"}, Score: "0"},
							{HomeAway: "away", Team: Team{Abbreviation: "DET", DisplayName: "Detroit Lions"}, Score: "0"},
						},
						Status: EventStatus{Type: StatusType{Name: "STATUS_SCHEDULED", Completed: false}},
					},
				},
			},
		},
	}
	body, _ := json.Marshal(sb)

	s := &Syncer{
		client: &Client{
			http: &http.Client{Transport: &mockTransport{statusCode: 200, body: string(body)}},
		},
		pool: pool,
	}

	w := &models.Week{ID: week.ID, WeekNumber: 1, SeasonYear: 2025}
	if err := s.SyncWeek(context.Background(), w, 2); err != nil {
		t.Fatalf("SyncWeek: %v", err)
	}

	// Verify game was upserted.
	var count int
	pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM games WHERE espn_game_id='espn-sync-1'`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 upserted game, got %d", count)
	}
}

func TestSyncWeek_FinalGameScoresPicks(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))

	sb := ScoreboardResponse{
		Events: []Event{
			{
				ID:   "espn-sync-final",
				Date: "2025-09-07T17:00:00Z",
				Competitions: []Competition{
					{
						Competitors: []Competitor{
							{HomeAway: "home", Team: Team{Abbreviation: "KC", DisplayName: "Kansas City Chiefs"}, Score: "24"},
							{HomeAway: "away", Team: Team{Abbreviation: "DET", DisplayName: "Detroit Lions"}, Score: "17"},
						},
						Status: EventStatus{Type: StatusType{Name: "STATUS_FINAL", Completed: true}},
					},
				},
			},
		},
	}
	body, _ := json.Marshal(sb)

	scoredCalled := false
	s := &Syncer{
		client: &Client{
			http: &http.Client{Transport: &mockTransport{statusCode: 200, body: string(body)}},
		},
		pool:     pool,
		onScored: func() { scoredCalled = true },
	}

	w := &models.Week{ID: week.ID, WeekNumber: 1, SeasonYear: 2025}
	if err := s.SyncWeek(context.Background(), w, 2); err != nil {
		t.Fatalf("SyncWeek: %v", err)
	}

	if !scoredCalled {
		t.Error("expected onScored callback to be called for final game")
	}
}

func TestSyncWeek_FetchError(t *testing.T) {
	pool := testutil.NewTestDB(t)
	s := &Syncer{
		client: &Client{
			http: &http.Client{Transport: &mockTransport{statusCode: 503, body: ""}},
		},
		pool: pool,
	}
	w := &models.Week{WeekNumber: 1, SeasonYear: 2025}
	if err := s.SyncWeek(context.Background(), w, 2); err == nil {
		t.Fatal("expected error for fetch failure")
	}
}
