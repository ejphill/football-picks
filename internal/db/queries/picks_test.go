package queries_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/evan/football-picks/internal/db/queries"
	"github.com/evan/football-picks/internal/testutil"
	"github.com/google/uuid"
)

func TestAutoScoring(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(-2*time.Hour))
	user := testutil.SeedUser(t, pool, "uid-score-1", "Scorer", "scorer@test.com")

	winner := "home"
	game := testutil.SeedGame(t, pool, week.ID, "espn-score-1", "KC", "DET", "final", &winner)

	cases := []struct {
		name        string
		pickedTeam  string
		wantCorrect bool
	}{
		{"correct home pick", "home", true},
		{"wrong away pick", "away", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Insert pick directly (bypassing UpsertPick to test ScorePicks in isolation).
			_, err := pool.Exec(context.Background(), `
				INSERT INTO picks (user_id, game_id, picked_team)
				VALUES ($1, $2, $3)
				ON CONFLICT (user_id, game_id) DO UPDATE SET picked_team = EXCLUDED.picked_team, is_correct = NULL, updated_at = NOW()
			`, user.ID, game.ID, tc.pickedTeam)
			if err != nil {
				t.Fatalf("insert pick: %v", err)
			}

			if err := queries.ScorePicks(context.Background(), pool); err != nil {
				t.Fatalf("ScorePicks: %v", err)
			}

			var isCorrect *bool
			err = pool.QueryRow(context.Background(), `
				SELECT is_correct FROM picks WHERE user_id = $1 AND game_id = $2
			`, user.ID, game.ID).Scan(&isCorrect)
			if err != nil {
				t.Fatalf("scan is_correct: %v", err)
			}

			if isCorrect == nil {
				t.Fatalf("is_correct is NULL, want %v", tc.wantCorrect)
			}
			if *isCorrect != tc.wantCorrect {
				t.Errorf("is_correct = %v, want %v", *isCorrect, tc.wantCorrect)
			}
		})
	}
}

func TestUpsertPickReplacesPrevious(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
	user := testutil.SeedUser(t, pool, "uid-upsert-1", "Upsert User", "upsert@test.com")
	game := testutil.SeedGame(t, pool, week.ID, "espn-upsert-1", "KC", "DET", "scheduled", nil)

	// First pick: home
	p1, created, err := queries.UpsertPick(context.Background(), pool, user.ID, game.ID, "home")
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if !created {
		t.Error("first pick should be created=true")
	}
	if p1.PickedTeam != "home" {
		t.Errorf("first pick: got %q, want %q", p1.PickedTeam, "home")
	}

	// Second pick: away — should replace first.
	p2, created, err := queries.UpsertPick(context.Background(), pool, user.ID, game.ID, "away")
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if created {
		t.Error("second pick should be created=false (update)")
	}
	if p2.PickedTeam != "away" {
		t.Errorf("second pick: got %q, want %q", p2.PickedTeam, "away")
	}
	if p2.ID != p1.ID {
		t.Errorf("upsert should return same row id; got %v, want %v", p2.ID, p1.ID)
	}

	// Confirm only one row in DB.
	var count int
	pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM picks WHERE user_id = $1 AND game_id = $2`, user.ID, game.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 pick row, got %d", count)
	}
}

func TestUpsertPickLockedAfterKickoff(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(-time.Hour))
	user := testutil.SeedUser(t, pool, "uid-lock-1", "Locked User", "locked@test.com")
	// Game has already kicked off.
	game := testutil.SeedGameAt(t, pool, week.ID, "espn-lock-1", "KC", "DET", "in_progress", nil, time.Now().Add(-30*time.Minute))

	_, _, err := queries.UpsertPick(context.Background(), pool, user.ID, game.ID, "home")
	if !errors.Is(err, queries.ErrPickLocked) {
		t.Errorf("UpsertPick after kickoff: got %v, want ErrPickLocked", err)
	}
}

func TestDeletePickLocked(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(-time.Hour))
	user := testutil.SeedUser(t, pool, "uid-lock-2", "Delete Lock", "deletelock@test.com")
	game := testutil.SeedGameAt(t, pool, week.ID, "espn-lock-2", "SF", "SEA", "in_progress", nil, time.Now().Add(-30*time.Minute))

	// Insert pick directly since UpsertPick also enforces the lock.
	_, err := pool.Exec(context.Background(), `
		INSERT INTO picks (user_id, game_id, picked_team) VALUES ($1, $2, 'home')
	`, user.ID, game.ID)
	if err != nil {
		t.Fatalf("insert pick: %v", err)
	}

	if err := queries.DeletePick(context.Background(), pool, user.ID, game.ID); !errors.Is(err, queries.ErrPickLocked) {
		t.Errorf("DeletePick after kickoff: got %v, want ErrPickLocked", err)
	}
}

func TestUpsertPickOnFinalGameScoresImmediately(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(-time.Hour))
	user := testutil.SeedUser(t, pool, "uid-final-1", "Final User", "final@test.com")

	winner := "away"
	game := testutil.SeedGame(t, pool, week.ID, "espn-final-1", "NE", "MIA", "final", &winner)

	pick, created, err := queries.UpsertPick(context.Background(), pool, user.ID, game.ID, "away")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if !created {
		t.Error("first pick on final game should be created=true")
	}
	if pick.IsCorrect == nil {
		t.Fatal("is_correct should be set on final game pick")
	}
	if !*pick.IsCorrect {
		t.Error("picking the winner should be correct")
	}
}

func TestGetPicksByUserAndWeek(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
	user := testutil.SeedUser(t, pool, "uid-gpuw-1", "GPUWUser", "gpuw@test.com")
	game := testutil.SeedGame(t, pool, week.ID, "espn-gpuw-1", "KC", "DET", "scheduled", nil)
	testutil.SeedPick(t, pool, user.ID, game.ID, "home")

	picks, err := queries.GetPicksByUserAndWeek(context.Background(), pool, user.ID, week.ID)
	if err != nil {
		t.Fatalf("GetPicksByUserAndWeek: %v", err)
	}
	if len(picks) != 1 {
		t.Fatalf("expected 1 pick, got %d", len(picks))
	}
	if picks[0].PickedTeam != "home" {
		t.Errorf("picked_team: got %q, want home", picks[0].PickedTeam)
	}
}

func TestGetPicksByUserAndWeek_Empty(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
	user := testutil.SeedUser(t, pool, "uid-gpuw-2", "NoPicksUser", "gpuw2@test.com")

	picks, err := queries.GetPicksByUserAndWeek(context.Background(), pool, user.ID, week.ID)
	if err != nil {
		t.Fatalf("GetPicksByUserAndWeek: %v", err)
	}
	if len(picks) != 0 {
		t.Errorf("expected 0 picks, got %d", len(picks))
	}
}

func TestGetAllPicksByWeek(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
	u1 := testutil.SeedUser(t, pool, "uid-gapw-1", "GAPWUser1", "gapw1@test.com")
	u2 := testutil.SeedUser(t, pool, "uid-gapw-2", "GAPWUser2", "gapw2@test.com")
	game := testutil.SeedGame(t, pool, week.ID, "espn-gapw-1", "KC", "DET", "scheduled", nil)
	pool.Exec(context.Background(), `UPDATE games SET included_in_picks=TRUE WHERE id=$1`, game.ID)

	testutil.SeedPick(t, pool, u1.ID, game.ID, "home")
	testutil.SeedPick(t, pool, u2.ID, game.ID, "away")

	rows, err := queries.GetAllPicksByWeek(context.Background(), pool, week.ID)
	if err != nil {
		t.Fatalf("GetAllPicksByWeek: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
}

func TestGetAllPicksByWeek_ExcludesNonIncluded(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
	user := testutil.SeedUser(t, pool, "uid-gapw-3", "GAPWUser3", "gapw3@test.com")
	game := testutil.SeedGame(t, pool, week.ID, "espn-gapw-2", "NE", "MIA", "scheduled", nil)
	// included_in_picks defaults to TRUE — explicitly exclude this game.
	pool.Exec(context.Background(), `UPDATE games SET included_in_picks=FALSE WHERE id=$1`, game.ID)
	testutil.SeedPick(t, pool, user.ID, game.ID, "home")

	rows, err := queries.GetAllPicksByWeek(context.Background(), pool, week.ID)
	if err != nil {
		t.Fatalf("GetAllPicksByWeek: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows (excluded), got %d", len(rows))
	}
}

func TestGetPicksForUsersAndWeek(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
	u1 := testutil.SeedUser(t, pool, "uid-gpfuw-1", "FPUser1", "fpuw1@test.com")
	u2 := testutil.SeedUser(t, pool, "uid-gpfuw-2", "FPUser2", "fpuw2@test.com")
	game := testutil.SeedGame(t, pool, week.ID, "espn-gpfuw-1", "KC", "DET", "scheduled", nil)
	pool.Exec(context.Background(), `UPDATE games SET included_in_picks=TRUE WHERE id=$1`, game.ID)

	testutil.SeedPick(t, pool, u1.ID, game.ID, "home")
	testutil.SeedPick(t, pool, u2.ID, game.ID, "away")

	// Only fetch for u1.
	rows, err := queries.GetPicksForUsersAndWeek(context.Background(), pool, week.ID, []uuid.UUID{u1.ID})
	if err != nil {
		t.Fatalf("GetPicksForUsersAndWeek: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].PickedTeam != "home" {
		t.Errorf("picked_team: got %q, want home", rows[0].PickedTeam)
	}
}

func TestGetPicksForUsersAndWeek_EmptyIDs(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	rows, err := queries.GetPicksForUsersAndWeek(context.Background(), pool, 1, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows != nil {
		t.Errorf("expected nil for empty userIDs, got %v", rows)
	}
}
