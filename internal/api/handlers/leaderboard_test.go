package handlers_test

import (
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
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestWeeklyLeaderboard(t *testing.T) {
	pool := testutil.NewTestDB(t)

	t.Run("returns weekly standings", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(-time.Hour))
		user := testutil.SeedUser(t, pool, "uid-wl-1", "WeeklyUser", "weekly@test.com")
		winner := "home"
		game := testutil.SeedGame(t, pool, week.ID, "espn-wl-1", "KC", "DET", "final", &winner)
		testutil.SeedPick(t, pool, user.ID, game.ID, "home")
		pool.Exec(context.Background(), `UPDATE picks SET is_correct=TRUE`)

		rr := doWeeklyLeaderboard(t, pool, user.SupabaseUID, 1, 2025)
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d — body: %s", rr.Code, rr.Body.String())
		}
		var body struct {
			Entries []map[string]interface{} `json:"entries"`
			Total   int                      `json:"total"`
			Locked  bool                     `json:"locked"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.Total != 1 {
			t.Errorf("total: got %d, want 1", body.Total)
		}
		if !body.Locked {
			t.Error("expected locked=true for past week")
		}
	})

	t.Run("before lock time — other users picks hidden", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(2*time.Hour))

		viewer := testutil.SeedUser(t, pool, "uid-lv-viewer", "Viewer", "viewer@test.com")
		other := testutil.SeedUser(t, pool, "uid-lv-other", "Other", "other@test.com")

		game := testutil.SeedGame(t, pool, week.ID, "espn-lv-1", "KC", "DET", "scheduled", nil)
		testutil.SeedPick(t, pool, viewer.ID, game.ID, "home")
		testutil.SeedPick(t, pool, other.ID, game.ID, "away")

		resp := doWeeklyLeaderboard(t, pool, viewer.SupabaseUID, 1, 2025)
		if resp.Code != http.StatusOK {
			t.Fatalf("status: %d — %s", resp.Code, resp.Body.String())
		}

		var body struct {
			Locked  bool                     `json:"locked"`
			Entries []map[string]interface{} `json:"entries"`
		}
		if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.Locked {
			t.Error("locked should be false before lock time")
		}

		for _, entry := range body.Entries {
			uid := fmt.Sprintf("%v", entry["user_id"])
			picksRaw, _ := json.Marshal(entry["picks"])
			var picks []map[string]interface{}
			json.Unmarshal(picksRaw, &picks)

			for _, p := range picks {
				pickedTeam := fmt.Sprintf("%v", p["picked_team"])
				if uid == viewer.ID.String() {
					if pickedTeam == "" {
						t.Error("viewer should see their own pick")
					}
				} else {
					if pickedTeam != "" {
						t.Errorf("other user's pick should be hidden before lock, got %q", pickedTeam)
					}
				}
			}
		}
	})

	t.Run("after lock time — all picks visible", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(-time.Hour))

		viewer := testutil.SeedUser(t, pool, "uid-lv-viewer2", "Viewer2", "viewer2@test.com")
		other := testutil.SeedUser(t, pool, "uid-lv-other2", "Other2", "other2@test.com")

		game := testutil.SeedGame(t, pool, week.ID, "espn-lv-2", "NE", "MIA", "scheduled", nil)
		testutil.SeedPick(t, pool, viewer.ID, game.ID, "home")
		testutil.SeedPick(t, pool, other.ID, game.ID, "away")

		resp := doWeeklyLeaderboard(t, pool, viewer.SupabaseUID, 1, 2025)
		if resp.Code != http.StatusOK {
			t.Fatalf("status: %d — %s", resp.Code, resp.Body.String())
		}

		var body struct {
			Locked  bool                     `json:"locked"`
			Entries []map[string]interface{} `json:"entries"`
		}
		if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !body.Locked {
			t.Error("locked should be true after lock time")
		}

		for _, entry := range body.Entries {
			picksRaw, _ := json.Marshal(entry["picks"])
			var picks []map[string]interface{}
			json.Unmarshal(picksRaw, &picks)
			for _, p := range picks {
				if fmt.Sprintf("%v", p["picked_team"]) == "" {
					t.Error("all picks should be visible after lock time")
				}
			}
		}
	})

	t.Run("400 for missing week param", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		user := testutil.SeedUser(t, pool, "uid-wl-2", "WLUser2", "wl2@test.com")
		rr := doWeeklyLeaderboardRaw(t, pool, user.SupabaseUID, "?season=2025")
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})

	t.Run("400 for missing season param", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		user := testutil.SeedUser(t, pool, "uid-wl-3", "WLUser3", "wl3@test.com")
		rr := doWeeklyLeaderboardRaw(t, pool, user.SupabaseUID, "?week=1")
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})

	t.Run("401 when user not found", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		rr := doWeeklyLeaderboardRaw(t, pool, "uid-nonexistent", "?week=1&season=2025")
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status: got %d, want 401", rr.Code)
		}
	})

	t.Run("404 for unknown week", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		user := testutil.SeedUser(t, pool, "uid-wl-4", "WLUser4", "wl4@test.com")
		rr := doWeeklyLeaderboard(t, pool, user.SupabaseUID, 99, 2025)
		if rr.Code != http.StatusNotFound {
			t.Errorf("status: got %d, want 404", rr.Code)
		}
	})

	t.Run("returns empty entries for week with no picks", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
		user := testutil.SeedUser(t, pool, "uid-wl-5", "WLUser5", "wl5@test.com")

		rr := doWeeklyLeaderboard(t, pool, user.SupabaseUID, 1, 2025)
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d — body: %s", rr.Code, rr.Body.String())
		}
		var body struct {
			Entries []interface{} `json:"entries"`
		}
		json.Unmarshal(rr.Body.Bytes(), &body)
		if len(body.Entries) != 0 {
			t.Errorf("expected empty entries, got %d", len(body.Entries))
		}
	})

	t.Run("respects limit and offset pagination", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
		user := testutil.SeedUser(t, pool, "uid-wl-6", "WLUser6", "wl6@test.com")

		rr := doWeeklyLeaderboardRaw(t, pool, user.SupabaseUID, "?week=1&season=2025&limit=5&offset=0")
		if rr.Code != http.StatusOK {
			t.Errorf("status: got %d, want 200", rr.Code)
		}
	})
}

func TestSeasonLeaderboard(t *testing.T) {
	pool := testutil.NewTestDB(t)

	t.Run("returns season standings", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(-time.Hour))
		user := testutil.SeedUser(t, pool, "uid-sl-1", "SeasonUser", "season@test.com")
		winner := "home"
		game := testutil.SeedGame(t, pool, week.ID, "espn-sl-1", "KC", "DET", "final", &winner)
		testutil.SeedPick(t, pool, user.ID, game.ID, "home")

		pool.Exec(context.Background(), `UPDATE picks SET is_correct=TRUE`)

		rr := doSeasonLeaderboard(t, pool, 2025)
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d — body: %s", rr.Code, rr.Body.String())
		}
		var body struct {
			Entries []map[string]interface{} `json:"entries"`
			Total   int                      `json:"total"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.Total != 1 {
			t.Errorf("total: got %d, want 1", body.Total)
		}
	})

	t.Run("400 for missing season param", func(t *testing.T) {
		rr := doSeasonLeaderboardRaw(t, pool, "")
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})

	t.Run("returns empty entries for season with no picks", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		rr := doSeasonLeaderboard(t, pool, 2025)
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d — body: %s", rr.Code, rr.Body.String())
		}
		var body struct {
			Entries []interface{} `json:"entries"`
		}
		json.Unmarshal(rr.Body.Bytes(), &body)
		if len(body.Entries) != 0 {
			t.Errorf("expected empty entries, got %d", len(body.Entries))
		}
	})

	t.Run("respects limit and offset pagination", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		rr := doSeasonLeaderboardRaw(t, pool, "?season=2025&limit=10&offset=0")
		if rr.Code != http.StatusOK {
			t.Errorf("status: got %d, want 200", rr.Code)
		}
	})
}

func doWeeklyLeaderboard(t *testing.T, pool *pgxpool.Pool, uid string, week, season int) *httptest.ResponseRecorder {
	t.Helper()
	return doWeeklyLeaderboardRaw(t, pool, uid, fmt.Sprintf("?week=%d&season=%d", week, season))
}

func doWeeklyLeaderboardRaw(t *testing.T, pool *pgxpool.Pool, uid, query string) *httptest.ResponseRecorder {
	t.Helper()
	lbH := handlers.NewLeaderboardHandler(pool, cache.NewUserCache(pool, nil), cache.NewLeaderboardCache(nil))
	r := httptest.NewRequest(http.MethodGet, "/api/v1/leaderboard/weekly"+query, nil)
	r = r.WithContext(context.WithValue(r.Context(), middleware.ContextKeySupabaseUID, uid))
	rr := httptest.NewRecorder()
	lbH.Weekly(rr, r)
	return rr
}

func doSeasonLeaderboard(t *testing.T, pool *pgxpool.Pool, season int) *httptest.ResponseRecorder {
	t.Helper()
	return doSeasonLeaderboardRaw(t, pool, fmt.Sprintf("?season=%d", season))
}

func doSeasonLeaderboardRaw(t *testing.T, pool *pgxpool.Pool, query string) *httptest.ResponseRecorder {
	t.Helper()
	lbH := handlers.NewLeaderboardHandler(pool, cache.NewUserCache(pool, nil), cache.NewLeaderboardCache(nil))
	r := httptest.NewRequest(http.MethodGet, "/api/v1/leaderboard/season"+query, nil)
	r = r.WithContext(context.WithValue(r.Context(), middleware.ContextKeySupabaseUID, "any-uid"))
	rr := httptest.NewRecorder()
	lbH.Season(rr, r)
	return rr
}
