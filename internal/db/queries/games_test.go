package queries_test

import (
	"context"
	"testing"
	"time"

	"github.com/evan/football-picks/internal/db/queries"
	"github.com/evan/football-picks/internal/models"
	"github.com/evan/football-picks/internal/testutil"
)

func TestGetGamesByWeek_OnlyReturnsIncluded(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
	// SeedGame seeds games with included_in_picks=TRUE (Saturday kickoff).
	testutil.SeedGame(t, pool, week.ID, "espn-ggw-1", "KC", "DET", "scheduled", nil)
	testutil.SeedGame(t, pool, week.ID, "espn-ggw-2", "NE", "MIA", "scheduled", nil)

	// Exclude one game manually.
	pool.Exec(context.Background(),
		`UPDATE games SET included_in_picks=FALSE WHERE espn_game_id='espn-ggw-2'`)

	games, err := queries.GetGamesByWeek(context.Background(), pool, week.ID)
	if err != nil {
		t.Fatalf("GetGamesByWeek: %v", err)
	}
	if len(games) != 1 {
		t.Fatalf("expected 1 included game, got %d", len(games))
	}
	if games[0].ESPNGameID != "espn-ggw-1" {
		t.Errorf("unexpected game: %q", games[0].ESPNGameID)
	}
}

func TestGetAllGamesByWeek_ReturnsAll(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
	testutil.SeedGame(t, pool, week.ID, "espn-agw-1", "KC", "DET", "scheduled", nil)
	testutil.SeedGame(t, pool, week.ID, "espn-agw-2", "NE", "MIA", "scheduled", nil)

	pool.Exec(context.Background(),
		`UPDATE games SET included_in_picks=FALSE WHERE espn_game_id='espn-agw-2'`)

	games, err := queries.GetAllGamesByWeek(context.Background(), pool, week.ID)
	if err != nil {
		t.Fatalf("GetAllGamesByWeek: %v", err)
	}
	if len(games) != 2 {
		t.Errorf("expected 2 games (all), got %d", len(games))
	}
}

func TestGetGameByID(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
	game := testutil.SeedGame(t, pool, week.ID, "espn-ggid-1", "KC", "DET", "scheduled", nil)

	got, err := queries.GetGameByID(context.Background(), pool, game.ID)
	if err != nil {
		t.Fatalf("GetGameByID: %v", err)
	}
	if got.ID != game.ID {
		t.Errorf("ID mismatch: got %v, want %v", got.ID, game.ID)
	}
	if got.HomeTeam != "KC" {
		t.Errorf("home_team: got %q, want KC", got.HomeTeam)
	}
}

func TestUpsertGame_InsertAndUpdate(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))

	g := &models.Game{
		WeekID:       week.ID,
		ESPNGameID:   "espn-upsert-g1",
		HomeTeam:     "KC",
		AwayTeam:     "DET",
		HomeTeamName: "Kansas City Chiefs",
		AwayTeamName: "Detroit Lions",
		KickoffAt:    time.Now().Add(2 * time.Hour),
		Status:       "scheduled",
	}

	// Insert.
	inserted, err := queries.UpsertGame(context.Background(), pool, g)
	if err != nil {
		t.Fatalf("UpsertGame insert: %v", err)
	}
	if inserted.HomeTeam != "KC" {
		t.Errorf("home_team after insert: got %q, want KC", inserted.HomeTeam)
	}

	// Update — change status.
	g.Status = "in_progress"
	updated, err := queries.UpsertGame(context.Background(), pool, g)
	if err != nil {
		t.Fatalf("UpsertGame update: %v", err)
	}
	if updated.Status != "in_progress" {
		t.Errorf("status after update: got %q, want in_progress", updated.Status)
	}
	if updated.ID != inserted.ID {
		t.Error("ID should not change on upsert conflict")
	}
}

func TestSetGameIncluded(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
	game := testutil.SeedGame(t, pool, week.ID, "espn-sgi-1", "KC", "DET", "scheduled", nil)

	// Exclude.
	updated, err := queries.SetGameIncluded(context.Background(), pool, game.ID, false)
	if err != nil {
		t.Fatalf("SetGameIncluded false: %v", err)
	}
	if updated.IncludedInPicks {
		t.Error("included_in_picks should be false")
	}

	// Re-include.
	updated, err = queries.SetGameIncluded(context.Background(), pool, game.ID, true)
	if err != nil {
		t.Fatalf("SetGameIncluded true: %v", err)
	}
	if !updated.IncludedInPicks {
		t.Error("included_in_picks should be true after re-include")
	}
}

func TestGetWeekByID(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 3, time.Now().Add(time.Hour))

	got, err := queries.GetWeekByID(context.Background(), pool, week.ID)
	if err != nil {
		t.Fatalf("GetWeekByID: %v", err)
	}
	if got.WeekNumber != 3 {
		t.Errorf("week_number: got %d, want 3", got.WeekNumber)
	}
	if got.SeasonYear != 2025 {
		t.Errorf("season_year: got %d, want 2025", got.SeasonYear)
	}
}

func TestGetWeekByNumberAndSeason(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	testutil.SeedWeek(t, pool, season.ID, 5, time.Now().Add(time.Hour))

	got, err := queries.GetWeekByNumberAndSeason(context.Background(), pool, 5, 2025)
	if err != nil {
		t.Fatalf("GetWeekByNumberAndSeason: %v", err)
	}
	if got.WeekNumber != 5 {
		t.Errorf("week_number: got %d, want 5", got.WeekNumber)
	}
}

func TestGetActiveWeek_ReturnsFutureLockFirst(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	// Week 1 already locked, week 2 still open.
	testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(-time.Hour))
	testutil.SeedWeek(t, pool, season.ID, 2, time.Now().Add(2*time.Hour))

	got, err := queries.GetActiveWeek(context.Background(), pool)
	if err != nil {
		t.Fatalf("GetActiveWeek: %v", err)
	}
	// Open week (lock in future) should be preferred.
	if got.WeekNumber != 2 {
		t.Errorf("week_number: got %d, want 2 (open week)", got.WeekNumber)
	}
}

func TestScorePicks_UpdatesIsCorrect(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(-time.Hour))
	u := testutil.SeedUser(t, pool, "uid-sp-1", "ScoreUser", "scoreuser@test.com")
	winner := "home"
	game := testutil.SeedGame(t, pool, week.ID, "espn-sp-1", "KC", "DET", "final", &winner)
	testutil.SeedPick(t, pool, u.ID, game.ID, "home")

	if err := queries.ScorePicks(context.Background(), pool); err != nil {
		t.Fatalf("ScorePicks: %v", err)
	}

	var isCorrect *bool
	pool.QueryRow(context.Background(),
		`SELECT is_correct FROM picks WHERE user_id=$1 AND game_id=$2`,
		u.ID, game.ID).Scan(&isCorrect)
	if isCorrect == nil || !*isCorrect {
		t.Error("pick should be marked correct")
	}
}
