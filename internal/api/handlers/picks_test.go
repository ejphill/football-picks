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
	"github.com/evan/football-picks/internal/testutil"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPicksSubmit(t *testing.T) {
	pool := testutil.NewTestDB(t)

	t.Run("creates pick successfully", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
		user := testutil.SeedUser(t, pool, "uid-sub-1", "SubUser", "sub@test.com")
		game := testutil.SeedGame(t, pool, week.ID, "espn-sub-1", "KC", "DET", "scheduled", nil)

		rr := doPicksSubmit(t, pool, user.SupabaseUID, game.ID.String(), "home")
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d — body: %s", rr.Code, rr.Body.String())
		}
		result := decodeSingleResult(t, rr)
		if result.Pick == nil || result.Error != "" {
			t.Fatalf("expected saved pick, got %+v", result)
		}
	})

	t.Run("accepts pick on in-progress game", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(-time.Hour))
		user := testutil.SeedUser(t, pool, "uid-ps-2", "Player2", "p2@test.com")
		game := testutil.SeedGame(t, pool, week.ID, "espn-ps-2", "KC", "DET", "in_progress", nil)

		rr := doPicksSubmit(t, pool, user.SupabaseUID, game.ID.String(), "away")
		result := decodeSingleResult(t, rr)
		if result.Pick == nil || result.Error != "" {
			t.Errorf("expected saved pick — body: %s", rr.Body.String())
		}
	})

	t.Run("accepts pick on final game and scores immediately", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(-2*time.Hour))
		user := testutil.SeedUser(t, pool, "uid-ps-3", "Player3", "p3@test.com")
		winner := "home"
		game := testutil.SeedGame(t, pool, week.ID, "espn-ps-3", "KC", "DET", "final", &winner)

		rr := doPicksSubmit(t, pool, user.SupabaseUID, game.ID.String(), "home")
		result := decodeSingleResult(t, rr)
		if result.Pick == nil {
			t.Fatalf("expected saved pick — body: %s", rr.Body.String())
		}
		if result.Pick.IsCorrect == nil {
			t.Fatal("is_correct should be set on final game")
		}
		if !*result.Pick.IsCorrect {
			t.Error("picking the winner should be correct")
		}
	})

	t.Run("upserts — second pick replaces first", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
		user := testutil.SeedUser(t, pool, "uid-ps-5", "Player5", "p5@test.com")
		game := testutil.SeedGame(t, pool, week.ID, "espn-ps-5", "NE", "MIA", "scheduled", nil)

		doPicksSubmit(t, pool, user.SupabaseUID, game.ID.String(), "home")
		rr2 := doPicksSubmit(t, pool, user.SupabaseUID, game.ID.String(), "away")

		result := decodeSingleResult(t, rr2)
		if result.Pick == nil || result.Pick.PickedTeam != "away" {
			t.Errorf("pick should be updated to 'away', got %+v", result)
		}
	})

	t.Run("401 when user not found", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		rr := doPicksSubmit(t, pool, "uid-nonexistent", "00000000-0000-0000-0000-000000000000", "home")
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status: got %d, want 401", rr.Code)
		}
	})

	t.Run("invalid game_id reported per-item, other picks unaffected", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
		user := testutil.SeedUser(t, pool, "uid-sub-2", "SubUser2", "sub2@test.com")
		game := testutil.SeedGame(t, pool, week.ID, "espn-sub-2b", "NE", "MIA", "scheduled", nil)

		rr := doPicksSubmitBatch(t, pool, user.SupabaseUID, []pickInput{
			{GameID: "not-a-uuid", PickedTeam: "home"},
			{GameID: game.ID.String(), PickedTeam: "home"},
		})
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d, want 200 — body: %s", rr.Code, rr.Body.String())
		}
		results := decodeResults(t, rr)
		if results[0].Error == "" || results[0].Pick != nil {
			t.Errorf("item 0: expected error, got %+v", results[0])
		}
		if results[1].Pick == nil {
			t.Errorf("item 1: expected saved pick despite item 0's error, got %+v", results[1])
		}
	})

	t.Run("invalid picked_team reported per-item", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
		user := testutil.SeedUser(t, pool, "uid-sub-3", "SubUser3", "sub3@test.com")
		game := testutil.SeedGame(t, pool, week.ID, "espn-sub-2", "KC", "DET", "scheduled", nil)

		rr := doPicksSubmit(t, pool, user.SupabaseUID, game.ID.String(), "invalid")
		result := decodeSingleResult(t, rr)
		if result.Error == "" || result.Pick != nil {
			t.Errorf("expected error, got %+v", result)
		}
	})

	t.Run("unknown game reported per-item", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		user := testutil.SeedUser(t, pool, "uid-sub-4", "SubUser4", "sub4@test.com")
		rr := doPicksSubmit(t, pool, user.SupabaseUID, "00000000-0000-0000-0000-000000000001", "home")
		result := decodeSingleResult(t, rr)
		if result.Error == "" || result.Pick != nil {
			t.Errorf("expected error, got %+v", result)
		}
	})

	t.Run("locked game reported per-item, other picks unaffected", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
		user := testutil.SeedUser(t, pool, "uid-sub-5", "SubUser5", "sub5@test.com")
		lockedGame := testutil.SeedGameAt(t, pool, week.ID, "espn-sub-3", "KC", "DET", "in_progress", nil, time.Now().Add(-30*time.Minute))
		openGame := testutil.SeedGame(t, pool, week.ID, "espn-sub-3b", "NE", "MIA", "scheduled", nil)

		rr := doPicksSubmitBatch(t, pool, user.SupabaseUID, []pickInput{
			{GameID: lockedGame.ID.String(), PickedTeam: "home"},
			{GameID: openGame.ID.String(), PickedTeam: "home"},
		})
		results := decodeResults(t, rr)
		if results[0].Error != "picks are locked for this game" {
			t.Errorf("item 0: got error %q, want lock error", results[0].Error)
		}
		if results[1].Pick == nil {
			t.Errorf("item 1: expected saved pick despite item 0 being locked, got %+v", results[1])
		}
	})

	t.Run("excluded game reported per-item, not saved", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
		user := testutil.SeedUser(t, pool, "uid-sub-8", "SubUser8", "sub8@test.com")
		game := testutil.SeedGame(t, pool, week.ID, "espn-sub-4", "KC", "DET", "scheduled", nil)
		pool.Exec(context.Background(), `UPDATE games SET included_in_picks = FALSE WHERE id = $1`, game.ID)

		rr := doPicksSubmit(t, pool, user.SupabaseUID, game.ID.String(), "home")
		result := decodeSingleResult(t, rr)
		if result.Error != "this game is not part of picks this week" {
			t.Errorf("got error %q, want exclusion error", result.Error)
		}
		if result.Pick != nil {
			t.Error("excluded game's pick should not be saved")
		}
	})

	t.Run("400 for invalid json body", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		user := testutil.SeedUser(t, pool, "uid-sub-6", "SubUser6", "sub6@test.com")
		rr := doPicksSubmitRaw(t, pool, user.SupabaseUID, []byte("not json"))
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})

	t.Run("400 for empty batch", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		user := testutil.SeedUser(t, pool, "uid-sub-7", "SubUser7", "sub7@test.com")
		rr := doPicksSubmitBatch(t, pool, user.SupabaseUID, []pickInput{})
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})
}

func TestPicksList(t *testing.T) {
	pool := testutil.NewTestDB(t)

	t.Run("returns picks for user and week", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
		user := testutil.SeedUser(t, pool, "uid-pl-1", "ListUser", "listuser@test.com")
		game := testutil.SeedGame(t, pool, week.ID, "espn-pl-1", "KC", "DET", "scheduled", nil)
		testutil.SeedPick(t, pool, user.ID, game.ID, "home")

		rr := doPicksList(t, pool, user.SupabaseUID, 1, 2025)
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d — body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("returns empty array when no picks", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
		user := testutil.SeedUser(t, pool, "uid-pl-2", "NoPicks", "nopicks@test.com")

		rr := doPicksList(t, pool, user.SupabaseUID, 1, 2025)
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d — body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("401 when user not found", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		rr := doPicksList(t, pool, "uid-nonexistent", 1, 2025)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status: got %d, want 401", rr.Code)
		}
	})

	t.Run("400 for missing week param", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		user := testutil.SeedUser(t, pool, "uid-pl-3", "ParamUser", "param@test.com")
		rr := doPicksListRaw(t, pool, user.SupabaseUID, "?season=2025")
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})

	t.Run("400 for missing season param", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		user := testutil.SeedUser(t, pool, "uid-pl-3b", "SeasonUser", "season@test.com")
		rr := doPicksListRaw(t, pool, user.SupabaseUID, "?week=1")
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})

	t.Run("404 for unknown week", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		user := testutil.SeedUser(t, pool, "uid-pl-4", "NoWeek", "noweek@test.com")
		rr := doPicksList(t, pool, user.SupabaseUID, 99, 2025)
		if rr.Code != http.StatusNotFound {
			t.Errorf("status: got %d, want 404", rr.Code)
		}
	})
}

func TestPicksDelete(t *testing.T) {
	pool := testutil.NewTestDB(t)

	t.Run("deletes existing pick", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(2*time.Hour))
		user := testutil.SeedUser(t, pool, "uid-pd-1", "DelUser", "del@test.com")
		game := testutil.SeedGame(t, pool, week.ID, "espn-pd-1", "KC", "DET", "scheduled", nil)
		testutil.SeedPick(t, pool, user.ID, game.ID, "home")

		rr := doPicksDelete(t, pool, user.SupabaseUID, game.ID.String())
		if rr.Code != http.StatusNoContent {
			t.Fatalf("status: got %d — body: %s", rr.Code, rr.Body.String())
		}

		var count int
		pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM picks WHERE user_id=$1 AND game_id=$2`,
			user.ID, game.ID).Scan(&count)
		if count != 0 {
			t.Error("pick should be deleted")
		}
	})

	t.Run("404 for pick that doesn't exist", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
		user := testutil.SeedUser(t, pool, "uid-pd-2", "NoPickDel", "nopickdel@test.com")
		game := testutil.SeedGame(t, pool, week.ID, "espn-pd-2", "NE", "MIA", "scheduled", nil)

		rr := doPicksDelete(t, pool, user.SupabaseUID, game.ID.String())
		if rr.Code != http.StatusNotFound {
			t.Errorf("status: got %d, want 404", rr.Code)
		}
	})

	t.Run("423 for locked game", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(-time.Hour))
		user := testutil.SeedUser(t, pool, "uid-pd-3", "LockDel", "lockdel@test.com")
		game := testutil.SeedGameAt(t, pool, week.ID, "espn-pd-3", "KC", "DET", "in_progress", nil, time.Now().Add(-30*time.Minute))

		// Insert pick directly to bypass UpsertPick lock.
		pool.Exec(context.Background(),
			`INSERT INTO picks (user_id, game_id, picked_team) VALUES ($1, $2, 'home')`,
			user.ID, game.ID)

		rr := doPicksDelete(t, pool, user.SupabaseUID, game.ID.String())
		if rr.Code != http.StatusLocked {
			t.Errorf("status: got %d, want 423", rr.Code)
		}
	})

	t.Run("400 for invalid game uuid", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		user := testutil.SeedUser(t, pool, "uid-pd-4", "BadUUID", "baduuid@test.com")
		rr := doPicksDelete(t, pool, user.SupabaseUID, "not-a-uuid")
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})

	t.Run("401 when user not found", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		rr := doPicksDelete(t, pool, "uid-nonexistent", "00000000-0000-0000-0000-000000000000")
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status: got %d, want 401", rr.Code)
		}
	})
}

type pickInput struct {
	GameID     string `json:"game_id"`
	PickedTeam string `json:"picked_team"`
}

func doPicksSubmit(t *testing.T, pool *pgxpool.Pool, uid, gameID, pickedTeam string) *httptest.ResponseRecorder {
	t.Helper()
	return doPicksSubmitBatch(t, pool, uid, []pickInput{{GameID: gameID, PickedTeam: pickedTeam}})
}

func doPicksSubmitBatch(t *testing.T, pool *pgxpool.Pool, uid string, picks []pickInput) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(picks)
	return doPicksSubmitRaw(t, pool, uid, body)
}

func decodeResults(t *testing.T, rr *httptest.ResponseRecorder) []handlers.PickResult {
	t.Helper()
	var results []handlers.PickResult
	if err := json.Unmarshal(rr.Body.Bytes(), &results); err != nil {
		t.Fatalf("decode response: %v — body: %s", err, rr.Body.String())
	}
	return results
}

func decodeSingleResult(t *testing.T, rr *httptest.ResponseRecorder) handlers.PickResult {
	t.Helper()
	results := decodeResults(t, rr)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d — body: %s", len(results), rr.Body.String())
	}
	return results[0]
}

func doPicksSubmitRaw(t *testing.T, pool *pgxpool.Pool, uid string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	ph := handlers.NewPicksHandler(pool, cache.NewUserCache(pool, nil), cache.NewLeaderboardCache(nil))
	r := httptest.NewRequest(http.MethodPost, "/api/v1/picks", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = r.WithContext(context.WithValue(r.Context(), middleware.ContextKeySupabaseUID, uid))
	rr := httptest.NewRecorder()
	ph.Submit(rr, r)
	return rr
}

func doPicksList(t *testing.T, pool *pgxpool.Pool, uid string, week, season int) *httptest.ResponseRecorder {
	t.Helper()
	return doPicksListRaw(t, pool, uid, fmt.Sprintf("?week=%d&season=%d", week, season))
}

func doPicksListRaw(t *testing.T, pool *pgxpool.Pool, uid, query string) *httptest.ResponseRecorder {
	t.Helper()
	ph := handlers.NewPicksHandler(pool, cache.NewUserCache(pool, nil), cache.NewLeaderboardCache(nil))
	r := httptest.NewRequest(http.MethodGet, "/api/v1/picks"+query, nil)
	r = r.WithContext(context.WithValue(r.Context(), middleware.ContextKeySupabaseUID, uid))
	rr := httptest.NewRecorder()
	ph.List(rr, r)
	return rr
}

func doPicksDelete(t *testing.T, pool *pgxpool.Pool, uid, gameID string) *httptest.ResponseRecorder {
	t.Helper()
	ph := handlers.NewPicksHandler(pool, cache.NewUserCache(pool, nil), cache.NewLeaderboardCache(nil))
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/picks/"+gameID, nil)
	r = r.WithContext(context.WithValue(r.Context(), middleware.ContextKeySupabaseUID, uid))
	rr := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Delete("/api/v1/picks/{gameId}", ph.Delete)
	router.ServeHTTP(rr, r)
	return rr
}
