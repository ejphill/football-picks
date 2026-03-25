package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/evan/football-picks/internal/api/handlers"
	"github.com/evan/football-picks/internal/models"
	"github.com/evan/football-picks/internal/testutil"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestActiveWeek(t *testing.T) {
	pool := testutil.NewTestDB(t)

	t.Run("returns active week", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(2*time.Hour))

		rr := doActiveWeek(t, pool)
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d — body: %s", rr.Code, rr.Body.String())
		}
		var w models.Week
		if err := json.Unmarshal(rr.Body.Bytes(), &w); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if w.WeekNumber != 1 {
			t.Errorf("week number: got %d, want 1", w.WeekNumber)
		}
	})

	t.Run("404 when no active season", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		rr := doActiveWeek(t, pool)
		if rr.Code != http.StatusNotFound {
			t.Errorf("status: got %d, want 404", rr.Code)
		}
	})
}

func TestGamesList(t *testing.T) {
	pool := testutil.NewTestDB(t)

	t.Run("returns games for week", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
		testutil.SeedGame(t, pool, week.ID, "espn-gl-1", "KC", "DET", "scheduled", nil)
		testutil.SeedGame(t, pool, week.ID, "espn-gl-2", "NE", "MIA", "scheduled", nil)

		rr := doGamesList(t, pool, 1, 2025)
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d — body: %s", rr.Code, rr.Body.String())
		}
		var games []models.Game
		json.Unmarshal(rr.Body.Bytes(), &games)
		if len(games) != 2 {
			t.Errorf("expected 2 games, got %d", len(games))
		}
	})

	t.Run("returns empty array when no games", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))

		rr := doGamesList(t, pool, 1, 2025)
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d", rr.Code)
		}
		var games []models.Game
		json.Unmarshal(rr.Body.Bytes(), &games)
		if len(games) != 0 {
			t.Errorf("expected 0 games, got %d", len(games))
		}
	})

	t.Run("404 for unknown week", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		rr := doGamesList(t, pool, 99, 2025)
		if rr.Code != http.StatusNotFound {
			t.Errorf("status: got %d, want 404", rr.Code)
		}
	})

	t.Run("400 for missing week param", func(t *testing.T) {
		rr := doGamesListRaw(t, pool, "")
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})

	t.Run("400 for missing season param", func(t *testing.T) {
		rr := doGamesListRaw(t, pool, "?week=1")
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})
}

func TestGamesGet(t *testing.T) {
	pool := testutil.NewTestDB(t)

	t.Run("returns game by id", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
		game := testutil.SeedGame(t, pool, week.ID, "espn-gg-1", "KC", "DET", "scheduled", nil)

		rr := doGamesGet(t, pool, game.ID.String())
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d — body: %s", rr.Code, rr.Body.String())
		}
		var g models.Game
		json.Unmarshal(rr.Body.Bytes(), &g)
		if g.ID != game.ID {
			t.Errorf("game id mismatch")
		}
	})

	t.Run("404 for unknown game", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		rr := doGamesGet(t, pool, "00000000-0000-0000-0000-000000000000")
		if rr.Code != http.StatusNotFound {
			t.Errorf("status: got %d, want 404", rr.Code)
		}
	})

	t.Run("400 for invalid uuid", func(t *testing.T) {
		rr := doGamesGet(t, pool, "not-a-uuid")
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})
}

// helpers

func doActiveWeek(t *testing.T, pool *pgxpool.Pool) *httptest.ResponseRecorder {
	t.Helper()
	gh := handlers.NewGamesHandler(pool)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/weeks/active", nil)
	rr := httptest.NewRecorder()
	gh.ActiveWeek(rr, r)
	return rr
}

func doGamesList(t *testing.T, pool *pgxpool.Pool, week, season int) *httptest.ResponseRecorder {
	t.Helper()
	return doGamesListRaw(t, pool, fmt.Sprintf("?week=%d&season=%d", week, season))
}

func doGamesListRaw(t *testing.T, pool *pgxpool.Pool, query string) *httptest.ResponseRecorder {
	t.Helper()
	gh := handlers.NewGamesHandler(pool)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/games"+query, nil)
	rr := httptest.NewRecorder()
	gh.List(rr, r)
	return rr
}

func doGamesGet(t *testing.T, pool *pgxpool.Pool, gameID string) *httptest.ResponseRecorder {
	t.Helper()
	gh := handlers.NewGamesHandler(pool)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/games/"+gameID, nil)
	rr := httptest.NewRecorder()

	router := chi.NewRouter()
	router.Get("/api/v1/games/{gameId}", gh.Get)
	router.ServeHTTP(rr, r)
	return rr
}
