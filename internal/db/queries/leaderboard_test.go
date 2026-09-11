package queries_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/evan/football-picks/internal/db/queries"
	"github.com/evan/football-picks/internal/models"
	"github.com/evan/football-picks/internal/testutil"
)

// TestWeeklyLeaderboardFloorScoring verifies that a user who skipped an entire
// kickoff window receives floor credit equal to the minimum correct count
// achieved by any other picker in that window.
//
// Setup:
//   - Window 1 (3 h ago, 3 games): Alice picks all 3 correct; Bob picks 1 correct + 2 wrong.
//   - Window 2 (1 h ago, 1 game):  All three users pick correctly.
//   - Carol has no picks in Window 1 (she is in all_pickers via Window 2).
//
// Floor for Window 1 = MIN(3, 1) = 1 (Bob defines the floor).
// Carol's floor credit = GREATEST(0, LEAST(1 - 0, 3)) = 1.
//
// Expected leaderboard:
//
//	Alice: 3 (W1) + 1 (W2) = 4 correct, 4 total — no floor credit
//	Carol: 0 (W1) + 1 (W2) + 1 floor = 2 correct, 4 total
//	Bob:   1 (W1) + 1 (W2) = 2 correct, 4 total — no floor credit
func TestWeeklyLeaderboardFloorScoring(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(-4*time.Hour))

	alice := testutil.SeedUser(t, pool, "uid-floor-a", "Alice", "alice@floor.com")
	bob := testutil.SeedUser(t, pool, "uid-floor-b", "Bob", "bob@floor.com")
	carol := testutil.SeedUser(t, pool, "uid-floor-c", "Carol", "carol@floor.com")

	// Window 1: three games, all kicked off 3 h ago (same truncated hour).
	w1 := time.Now().Add(-3 * time.Hour).Truncate(time.Hour)
	winner := "home"
	g1 := testutil.SeedGameAt(t, pool, week.ID, "espn-fl-1", "KC", "DET", "final", &winner, w1)
	g2 := testutil.SeedGameAt(t, pool, week.ID, "espn-fl-2", "SF", "SEA", "final", &winner, w1)
	g3 := testutil.SeedGameAt(t, pool, week.ID, "espn-fl-3", "BUF", "MIA", "final", &winner, w1)

	// Window 2: one game, kicked off 1 h ago (different truncated hour).
	w2 := time.Now().Add(-1 * time.Hour).Truncate(time.Hour)
	g4 := testutil.SeedGameAt(t, pool, week.ID, "espn-fl-4", "PHI", "DAL", "final", &winner, w2)

	// Alice: all 3 W1 games correct, W2 correct.
	testutil.SeedPick(t, pool, alice.ID, g1.ID, "home")
	testutil.SeedPick(t, pool, alice.ID, g2.ID, "home")
	testutil.SeedPick(t, pool, alice.ID, g3.ID, "home")
	testutil.SeedPick(t, pool, alice.ID, g4.ID, "home")

	// Bob: 1 W1 correct, 2 W1 wrong, W2 correct.
	testutil.SeedPick(t, pool, bob.ID, g1.ID, "home") // correct
	testutil.SeedPick(t, pool, bob.ID, g2.ID, "away") // wrong
	testutil.SeedPick(t, pool, bob.ID, g3.ID, "away") // wrong
	testutil.SeedPick(t, pool, bob.ID, g4.ID, "home")

	// Carol: skips W1 entirely, picks W2 correctly.
	testutil.SeedPick(t, pool, carol.ID, g4.ID, "home")

	if err := queries.ScorePicks(context.Background(), pool); err != nil {
		t.Fatalf("ScorePicks: %v", err)
	}

	scores, err := queries.GetWeeklyLeaderboardScores(context.Background(), pool, week.ID)
	if err != nil {
		t.Fatalf("GetWeeklyLeaderboardScores: %v", err)
	}
	if len(scores) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(scores))
	}

	byName := make(map[string]queries.WeeklyScoreRow, 3)
	for _, s := range scores {
		byName[s.DisplayName] = s
	}

	// Alice: 4 correct, 4 total (no floor credit needed).
	if byName["Alice"].Correct != 4 {
		t.Errorf("Alice correct: got %d, want 4", byName["Alice"].Correct)
	}
	// Bob: 2 correct, 4 total (no floor credit — he picked in W1).
	if byName["Bob"].Correct != 2 {
		t.Errorf("Bob correct: got %d, want 2", byName["Bob"].Correct)
	}
	// Carol: 1 actual + 1 floor credit = 2 correct, 1 actual + 3 floor = 4 total.
	if byName["Carol"].Correct != 2 {
		t.Errorf("Carol correct: got %d, want 2 (1 actual + 1 floor credit)", byName["Carol"].Correct)
	}
	if byName["Carol"].Total != 4 {
		t.Errorf("Carol total: got %d, want 4 (1 actual + 3 floor)", byName["Carol"].Total)
	}

	// Alice should rank first.
	if scores[0].DisplayName != "Alice" {
		t.Errorf("rank 1: got %q, want Alice", scores[0].DisplayName)
	}
}

// TestWeeklyLeaderboard_UnscoredPicksDontCountAsLosses verifies that a pick
// on a game which hasn't been scored yet doesn't count toward total in the
// weekly view either — mirrors the same fix in GetSeasonStandings.
func TestWeeklyLeaderboard_UnscoredPicksDontCountAsLosses(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
	user := testutil.SeedUser(t, pool, "uid-wk-unscored", "Dana", "dana@test.com")

	winner := "home"
	finalGame := testutil.SeedGameAt(t, pool, week.ID, "espn-wk-final", "KC", "DET", "final", &winner, time.Now().Add(-3*time.Hour))
	inProgressGame := testutil.SeedGameAt(t, pool, week.ID, "espn-wk-live", "SF", "SEA", "in_progress", nil, time.Now().Add(-30*time.Minute))

	testutil.SeedPick(t, pool, user.ID, finalGame.ID, "home")
	testutil.SeedPick(t, pool, user.ID, inProgressGame.ID, "home")

	if err := queries.ScorePicks(context.Background(), pool); err != nil {
		t.Fatalf("score picks: %v", err)
	}

	scores, err := queries.GetWeeklyLeaderboardScores(context.Background(), pool, week.ID)
	if err != nil {
		t.Fatalf("GetWeeklyLeaderboardScores: %v", err)
	}
	if len(scores) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(scores))
	}

	// The in-progress game's pick isn't scored yet, so it must not count —
	// only the final, correct pick should show up.
	if scores[0].Correct != 1 {
		t.Errorf("Correct: got %d, want 1", scores[0].Correct)
	}
	if scores[0].Total != 1 {
		t.Errorf("Total: got %d, want 1 (unscored pick should not count)", scores[0].Total)
	}
}

func TestSeasonLeaderboardOrder(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(-time.Hour))

	// User A: 8 correct out of 10 picks.
	userA := testutil.SeedUser(t, pool, "uid-lb-a", "Alice", "alice@test.com")
	// User B: 8 correct out of 9 picks — better tiebreaker (fewer total picks).
	userB := testutil.SeedUser(t, pool, "uid-lb-b", "Bob", "bob@test.com")

	winner := "home"
	games := make([]*models.Game, 10)
	for i := 0; i < 10; i++ {
		games[i] = testutil.SeedGame(t, pool, week.ID, fmt.Sprintf("espn-lb-%d", i), "KC", "DET", "final", &winner)
	}

	// Alice: picks on all 10 games — 8 correct ("home"), 2 wrong ("away").
	for i, g := range games {
		pick := "home"
		if i >= 8 {
			pick = "away"
		}
		testutil.SeedPick(t, pool, userA.ID, g.ID, pick)
	}

	// Bob: picks on first 9 games — 8 correct ("home"), 1 wrong ("away").
	for i, g := range games[:9] {
		pick := "home"
		if i >= 8 {
			pick = "away"
		}
		testutil.SeedPick(t, pool, userB.ID, g.ID, pick)
	}

	if err := queries.ScorePicks(context.Background(), pool); err != nil {
		t.Fatalf("score picks: %v", err)
	}

	standings, err := queries.GetSeasonStandings(context.Background(), pool, 2025)
	if err != nil {
		t.Fatalf("get standings: %v", err)
	}
	if len(standings) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(standings))
	}

	// Bob should rank first: same correct count, fewer total picks.
	if standings[0].DisplayName != "Bob" {
		t.Errorf("rank 1: got %q, want Bob", standings[0].DisplayName)
	}
	if standings[1].DisplayName != "Alice" {
		t.Errorf("rank 2: got %q, want Alice", standings[1].DisplayName)
	}
	if standings[0].Correct != 8 || standings[1].Correct != 8 {
		t.Errorf("both should have 8 correct; got Bob=%d Alice=%d", standings[0].Correct, standings[1].Correct)
	}
	if standings[0].Total != 9 {
		t.Errorf("Bob total: got %d, want 9", standings[0].Total)
	}
	if standings[1].Total != 10 {
		t.Errorf("Alice total: got %d, want 10", standings[1].Total)
	}
}

// TestSeasonStandings_UnscoredPicksDontCountAsLosses verifies that a pick on
// a game which hasn't been scored yet (is_correct still NULL) doesn't count
// toward total — it should be neither a win nor a loss until the game is
// actually final and ScorePicks has run.
func TestSeasonStandings_UnscoredPicksDontCountAsLosses(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
	user := testutil.SeedUser(t, pool, "uid-lb-unscored", "Carol", "carol@test.com")

	winner := "home"
	finalGame := testutil.SeedGame(t, pool, week.ID, "espn-lb-final", "KC", "DET", "final", &winner)
	scheduledGame := testutil.SeedGame(t, pool, week.ID, "espn-lb-scheduled", "NE", "MIA", "scheduled", nil)

	testutil.SeedPick(t, pool, user.ID, finalGame.ID, "home")
	testutil.SeedPick(t, pool, user.ID, scheduledGame.ID, "home")

	if err := queries.ScorePicks(context.Background(), pool); err != nil {
		t.Fatalf("score picks: %v", err)
	}

	standings, err := queries.GetSeasonStandings(context.Background(), pool, 2025)
	if err != nil {
		t.Fatalf("get standings: %v", err)
	}
	if len(standings) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(standings))
	}

	// Only the scored, correct pick should count — the unscored pick on
	// scheduledGame must not inflate Total (which would show as a loss).
	if standings[0].Correct != 1 {
		t.Errorf("Correct: got %d, want 1", standings[0].Correct)
	}
	if standings[0].Total != 1 {
		t.Errorf("Total: got %d, want 1 (unscored pick should not count)", standings[0].Total)
	}
}
