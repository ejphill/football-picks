package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/evan/football-picks/internal/api/handlers"
	"github.com/evan/football-picks/internal/api/middleware"
	"github.com/evan/football-picks/internal/cache"
	"github.com/evan/football-picks/internal/models"
	"github.com/evan/football-picks/internal/notify"
	"github.com/evan/football-picks/internal/testutil"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAdminScoreWeek(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(-time.Hour))
	user := testutil.SeedUser(t, pool, "uid-sw-1", "Scorer", "scorer-admin@test.com")
	winner := "home"
	game := testutil.SeedGame(t, pool, week.ID, "espn-sw-1", "KC", "DET", "final", &winner)
	testutil.SeedPick(t, pool, user.ID, game.ID, "home")

	rr := doAdminScoreWeek(t, pool)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d — body: %s", rr.Code, rr.Body.String())
	}

	var isCorrect *bool
	pool.QueryRow(context.Background(),
		`SELECT is_correct FROM picks WHERE user_id=$1 AND game_id=$2`,
		user.ID, game.ID).Scan(&isCorrect)
	if isCorrect == nil || !*isCorrect {
		t.Error("pick should be scored as correct after ScoreWeek")
	}
}

func TestAdminCreateAnnouncement(t *testing.T) {
	pool := testutil.NewTestDB(t)

	t.Run("creates announcement", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
		author := testutil.SeedUser(t, pool, "uid-ca-1", "Admin", "admin@test.com")

		rr := doAdminCreateAnnouncement(t, pool, author, 1, 2025, "Week 1 picks are open!")
		if rr.Code != http.StatusCreated {
			t.Fatalf("status: got %d — body: %s", rr.Code, rr.Body.String())
		}
		var a models.Announcement
		json.Unmarshal(rr.Body.Bytes(), &a)
		if a.Intro != "Week 1 picks are open!" {
			t.Errorf("intro: got %q", a.Intro)
		}
	})

	t.Run("rejects missing intro", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
		author := testutil.SeedUser(t, pool, "uid-ca-2", "Admin2", "admin2@test.com")

		rr := doAdminCreateAnnouncement(t, pool, author, 1, 2025, "")
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})

	t.Run("rejects missing week_number", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		author := testutil.SeedUser(t, pool, "uid-ca-3", "Admin3", "admin3@test.com")

		rr := doAdminCreateAnnouncement(t, pool, author, 0, 2025, "Some intro")
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})

	t.Run("rejects missing user in context", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		ah := handlers.NewAdminHandler(pool, nil, notify.NoopMailer{}, cache.NewLeaderboardCache(nil))
		body, _ := json.Marshal(map[string]interface{}{"week_id": 1, "intro": "hi"})
		r := httptest.NewRequest(http.MethodPost, "/api/v1/admin/announcements", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		ah.CreateAnnouncement(rr, r)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status: got %d, want 401", rr.Code)
		}
	})
}

func TestAdminListGames(t *testing.T) {
	pool := testutil.NewTestDB(t)

	t.Run("lists all games including non-included", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
		// SeedGame creates games with included_in_picks=TRUE by default (Saturday kickoff).
		testutil.SeedGame(t, pool, week.ID, "espn-ag-1", "KC", "DET", "scheduled", nil)
		testutil.SeedGame(t, pool, week.ID, "espn-ag-2", "NE", "MIA", "scheduled", nil)

		rr := doAdminListGames(t, pool, 1, 2025)
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d — body: %s", rr.Code, rr.Body.String())
		}
		var games []models.Game
		json.Unmarshal(rr.Body.Bytes(), &games)
		if len(games) != 2 {
			t.Errorf("expected 2 games, got %d", len(games))
		}
	})

	t.Run("404 for unknown week", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		rr := doAdminListGames(t, pool, 99, 2025)
		if rr.Code != http.StatusNotFound {
			t.Errorf("status: got %d, want 404", rr.Code)
		}
	})

	t.Run("400 for missing week param", func(t *testing.T) {
		rr := doAdminListGamesRaw(t, pool, "")
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})
}

func TestAdminUpdateGame(t *testing.T) {
	pool := testutil.NewTestDB(t)

	t.Run("toggles included_in_picks", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
		game := testutil.SeedGame(t, pool, week.ID, "espn-ug-1", "KC", "DET", "scheduled", nil)

		rr := doAdminUpdateGame(t, pool, game.ID.String(), false)
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d — body: %s", rr.Code, rr.Body.String())
		}
		var g models.Game
		json.Unmarshal(rr.Body.Bytes(), &g)
		if g.IncludedInPicks {
			t.Error("included_in_picks should be false after update")
		}
	})

	t.Run("400 for invalid uuid", func(t *testing.T) {
		rr := doAdminUpdateGame(t, pool, "not-a-uuid", false)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})

	t.Run("400 for missing included_in_picks field", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
		game := testutil.SeedGame(t, pool, week.ID, "espn-ug-2", "SF", "SEA", "scheduled", nil)

		ah := handlers.NewAdminHandler(pool, nil, notify.NoopMailer{}, cache.NewLeaderboardCache(nil))
		body, _ := json.Marshal(map[string]string{})
		r := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/games/"+game.ID.String(), bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router := chi.NewRouter()
		router.Patch("/api/v1/admin/games/{gameId}", ah.UpdateGame)
		router.ServeHTTP(rr, r)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})
}

func TestAdminDraftAnnouncement(t *testing.T) {
	pool := testutil.NewTestDB(t)

	t.Run("returns draft for week 1", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
		testutil.SeedGame(t, pool, week.ID, "espn-da-1", "KC", "DET", "scheduled", nil)

		rr := doAdminDraftAnnouncement(t, pool, 1, 2025)
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d — body: %s", rr.Code, rr.Body.String())
		}
		var draft map[string]string
		json.Unmarshal(rr.Body.Bytes(), &draft)
		if _, ok := draft["intro"]; !ok {
			t.Error("draft response should have intro field")
		}
	})

	t.Run("404 for unknown week", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		rr := doAdminDraftAnnouncement(t, pool, 99, 2025)
		if rr.Code != http.StatusNotFound {
			t.Errorf("status: got %d, want 404", rr.Code)
		}
	})

	t.Run("400 for missing week param", func(t *testing.T) {
		rr := doAdminDraftAnnouncementRaw(t, pool, "?season=2025")
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})

	t.Run("400 for missing season param", func(t *testing.T) {
		rr := doAdminDraftAnnouncementRaw(t, pool, "?week=1")
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})
}

func TestAdminSyncGames(t *testing.T) {
	pool := testutil.NewTestDB(t)

	t.Run("400 for missing week param", func(t *testing.T) {
		rr := doAdminSyncGamesRaw(t, pool, "?season=2025")
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})

	t.Run("400 for missing season param", func(t *testing.T) {
		rr := doAdminSyncGamesRaw(t, pool, "?week=1")
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})

	t.Run("404 for unknown week", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		rr := doAdminSyncGamesRaw(t, pool, "?week=99&season=2025")
		if rr.Code != http.StatusNotFound {
			t.Errorf("status: got %d, want 404", rr.Code)
		}
	})
}

func TestAdminListGames_MissingSeasonParam(t *testing.T) {
	pool := testutil.NewTestDB(t)
	rr := doAdminListGamesRaw(t, pool, "?week=1")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

func TestAdminCreateAnnouncement_InvalidJSON(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)
	author := testutil.SeedUser(t, pool, "uid-ca-inv", "Author", "author-inv@test.com")

	ah := handlers.NewAdminHandler(pool, nil, notify.NoopMailer{}, cache.NewLeaderboardCache(nil))
	r := httptest.NewRequest(http.MethodPost, "/api/v1/admin/announcements", bytes.NewReader([]byte("not json")))
	r = r.WithContext(context.WithValue(r.Context(), middleware.ContextKeyUser, author))
	r.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	ah.CreateAnnouncement(rr, r)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

// helpers

func doAdminScoreWeek(t *testing.T, pool *pgxpool.Pool) *httptest.ResponseRecorder {
	t.Helper()
	ah := handlers.NewAdminHandler(pool, nil, notify.NoopMailer{}, cache.NewLeaderboardCache(nil))
	r := httptest.NewRequest(http.MethodPost, "/api/v1/admin/score-week", nil)
	rr := httptest.NewRecorder()
	ah.ScoreWeek(rr, r)
	return rr
}

func doAdminCreateAnnouncement(t *testing.T, pool *pgxpool.Pool, author *models.User, weekNumber, seasonYear int, intro string) *httptest.ResponseRecorder {
	t.Helper()
	ah := handlers.NewAdminHandler(pool, nil, notify.NoopMailer{}, cache.NewLeaderboardCache(nil))
	body, _ := json.Marshal(map[string]interface{}{"week_number": weekNumber, "season_year": seasonYear, "intro": intro})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/admin/announcements", bytes.NewReader(body))
	r = r.WithContext(context.WithValue(r.Context(), middleware.ContextKeyUser, author))
	r.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	ah.CreateAnnouncement(rr, r)
	return rr
}

func doAdminListGames(t *testing.T, pool *pgxpool.Pool, week, season int) *httptest.ResponseRecorder {
	t.Helper()
	return doAdminListGamesRaw(t, pool, fmt.Sprintf("?week=%d&season=%d", week, season))
}

func doAdminListGamesRaw(t *testing.T, pool *pgxpool.Pool, query string) *httptest.ResponseRecorder {
	t.Helper()
	ah := handlers.NewAdminHandler(pool, nil, notify.NoopMailer{}, cache.NewLeaderboardCache(nil))
	r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/games"+query, nil)
	rr := httptest.NewRecorder()
	ah.ListGames(rr, r)
	return rr
}

func doAdminUpdateGame(t *testing.T, pool *pgxpool.Pool, gameID string, included bool) *httptest.ResponseRecorder {
	t.Helper()
	ah := handlers.NewAdminHandler(pool, nil, notify.NoopMailer{}, cache.NewLeaderboardCache(nil))
	body, _ := json.Marshal(map[string]bool{"included_in_picks": included})
	r := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/games/"+gameID, bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Patch("/api/v1/admin/games/{gameId}", ah.UpdateGame)
	router.ServeHTTP(rr, r)
	return rr
}

func doAdminDraftAnnouncement(t *testing.T, pool *pgxpool.Pool, week, season int) *httptest.ResponseRecorder {
	t.Helper()
	return doAdminDraftAnnouncementRaw(t, pool, fmt.Sprintf("?week=%d&season=%d", week, season))
}

func doAdminDraftAnnouncementRaw(t *testing.T, pool *pgxpool.Pool, query string) *httptest.ResponseRecorder {
	t.Helper()
	ah := handlers.NewAdminHandler(pool, nil, notify.NoopMailer{}, cache.NewLeaderboardCache(nil))
	r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/draft-announcement"+query, nil)
	rr := httptest.NewRecorder()
	ah.DraftAnnouncement(rr, r)
	return rr
}

func doAdminSyncGamesRaw(t *testing.T, pool *pgxpool.Pool, query string) *httptest.ResponseRecorder {
	t.Helper()
	ah := handlers.NewAdminHandler(pool, nil, notify.NoopMailer{}, cache.NewLeaderboardCache(nil))
	r := httptest.NewRequest(http.MethodPost, "/api/v1/admin/sync-games"+query, nil)
	rr := httptest.NewRecorder()
	ah.SyncGames(rr, r)
	return rr
}
