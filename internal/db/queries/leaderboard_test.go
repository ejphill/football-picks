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

	scores, err := queries.GetWeeklyLeaderboardScores(context.Background(), pool, 2025, week.ID)
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

	scores, err := queries.GetWeeklyLeaderboardScores(context.Background(), pool, 2025, week.ID)
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

// TestFloorScoring_FullyMissedWeek verifies that a user who misses an
// entire week (zero picks, not just a partial window) still gets a
// floor-credited row — as long as they've picked something elsewhere this
// season, distinguishing "an active player had a bad week" from "someone
// who's never played at all."
func TestFloorScoring_FullyMissedWeek(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week1 := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(-48*time.Hour))
	week2 := testutil.SeedWeek(t, pool, season.ID, 2, time.Now().Add(-time.Hour))

	alice := testutil.SeedUser(t, pool, "uid-fw-alice", "Alice", "alice@fw.com")
	bob := testutil.SeedUser(t, pool, "uid-fw-bob", "Bob", "bob@fw.com")

	// Week 1: both participate, so Bob is a season participant.
	winner := "home"
	w1g := testutil.SeedGameAt(t, pool, week1.ID, "espn-fw-w1", "KC", "DET", "final", &winner, time.Now().Add(-47*time.Hour))
	testutil.SeedPick(t, pool, alice.ID, w1g.ID, "home")
	testutil.SeedPick(t, pool, bob.ID, w1g.ID, "home")

	// Week 2: Alice picks both games (1 right, 1 wrong). Bob misses the
	// entire week — zero picks on anything.
	w2 := time.Now().Add(-2 * time.Hour).Truncate(time.Hour)
	g1 := testutil.SeedGameAt(t, pool, week2.ID, "espn-fw-w2a", "NE", "MIA", "final", &winner, w2)
	g2 := testutil.SeedGameAt(t, pool, week2.ID, "espn-fw-w2b", "SF", "SEA", "final", &winner, w2)
	testutil.SeedPick(t, pool, alice.ID, g1.ID, "home")  // correct
	testutil.SeedPick(t, pool, alice.ID, g2.ID, "away")  // wrong

	if err := queries.ScorePicks(context.Background(), pool); err != nil {
		t.Fatalf("score picks: %v", err)
	}

	scores, err := queries.GetWeeklyLeaderboardScores(context.Background(), pool, 2025, week2.ID)
	if err != nil {
		t.Fatalf("GetWeeklyLeaderboardScores: %v", err)
	}
	if len(scores) != 2 {
		t.Fatalf("expected 2 entries (Bob should still appear despite 0 picks), got %d", len(scores))
	}

	byName := make(map[string]queries.WeeklyScoreRow, 2)
	for _, s := range scores {
		byName[s.DisplayName] = s
	}

	// Floor for week 2's window = MIN(correct) among actual submitters = Alice's 1.
	if byName["Bob"].Correct != 1 {
		t.Errorf("Bob correct: got %d, want 1 (floor credit for fully-missed week)", byName["Bob"].Correct)
	}
	if byName["Bob"].Total != 2 {
		t.Errorf("Bob total: got %d, want 2 (both games in the missed window credited)", byName["Bob"].Total)
	}
}

// TestGetWindowCredits verifies the per-window credit breakdown used to
// assign floor credit to specific game cells in the UI.
func TestGetWindowCredits(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(-time.Hour))

	alice := testutil.SeedUser(t, pool, "uid-wc-alice", "Alice", "alice@wc.com")
	bob := testutil.SeedUser(t, pool, "uid-wc-bob", "Bob", "bob@wc.com")

	winner := "home"
	wnd := time.Now().Add(-2 * time.Hour).Truncate(time.Hour)
	g1 := testutil.SeedGameAt(t, pool, week.ID, "espn-wc-1", "KC", "DET", "final", &winner, wnd)
	g2 := testutil.SeedGameAt(t, pool, week.ID, "espn-wc-2", "NE", "MIA", "final", &winner, wnd)
	g3 := testutil.SeedGameAt(t, pool, week.ID, "espn-wc-3", "SF", "SEA", "final", &winner, wnd)

	// Alice picks all 3, gets 2 right (floor = 2).
	testutil.SeedPick(t, pool, alice.ID, g1.ID, "home") // correct
	testutil.SeedPick(t, pool, alice.ID, g2.ID, "home") // correct
	testutil.SeedPick(t, pool, alice.ID, g3.ID, "away") // wrong

	// Bob skips this window entirely, but needs a pick somewhere this season
	// to be eligible for floor credit at all — a future, not-yet-started
	// game does that without touching the window being tested.
	future := testutil.SeedGameAt(t, pool, week.ID, "espn-wc-4", "LAC", "ARI", "scheduled", nil, time.Now().Add(2*time.Hour))
	testutil.SeedPick(t, pool, bob.ID, future.ID, "home")

	if err := queries.ScorePicks(context.Background(), pool); err != nil {
		t.Fatalf("score picks: %v", err)
	}

	credits, err := queries.GetWindowCredits(context.Background(), pool, 2025, week.ID)
	if err != nil {
		t.Fatalf("GetWindowCredits: %v", err)
	}

	var bobCredit *queries.WindowCreditRow
	for i := range credits {
		if credits[i].UserID == bob.ID {
			bobCredit = &credits[i]
		}
	}
	if bobCredit == nil {
		t.Fatalf("expected a credit row for Bob, got none (rows: %+v)", credits)
	}
	if bobCredit.CreditCorrect != 2 {
		t.Errorf("Bob credit_correct: got %d, want 2 (floor)", bobCredit.CreditCorrect)
	}
	if bobCredit.CreditTotal != 3 {
		t.Errorf("Bob credit_total: got %d, want 3 (all 3 games in the window)", bobCredit.CreditTotal)
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

// TestGetSeasonStandingsThroughWeek verifies that the "through week" variant
// excludes the given week's own games — e.g. the announcement's records
// section shouldn't already include this week's Sunday results just
// because someone checks it on Monday before Jack's sent anything.
func TestGetSeasonStandingsThroughWeek(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week1 := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(-48*time.Hour))
	week2 := testutil.SeedWeek(t, pool, season.ID, 2, time.Now().Add(-time.Hour))

	user := testutil.SeedUser(t, pool, "uid-tw-user", "Gary", "gary@tw.com")

	winner := "home"
	g1 := testutil.SeedGameAt(t, pool, week1.ID, "espn-tw-w1", "KC", "DET", "final", &winner, time.Now().Add(-47*time.Hour))
	g2 := testutil.SeedGameAt(t, pool, week2.ID, "espn-tw-w2", "NE", "MIA", "final", &winner, time.Now().Add(-2*time.Hour))

	testutil.SeedPick(t, pool, user.ID, g1.ID, "home") // week 1: correct
	testutil.SeedPick(t, pool, user.ID, g2.ID, "home") // week 2: correct, just scored

	if err := queries.ScorePicks(context.Background(), pool); err != nil {
		t.Fatalf("score picks: %v", err)
	}

	// Live standings include both weeks.
	live, err := queries.GetSeasonStandings(context.Background(), pool, 2025)
	if err != nil {
		t.Fatalf("get standings: %v", err)
	}
	if live[0].Correct != 2 || live[0].Total != 2 {
		t.Errorf("live standings: got %d/%d, want 2/2 (both weeks)", live[0].Correct, live[0].Total)
	}

	// "Through week 2" excludes week 2 itself — only week 1 counts.
	frozen, err := queries.GetSeasonStandingsThroughWeek(context.Background(), pool, 2025, 2)
	if err != nil {
		t.Fatalf("get standings through week: %v", err)
	}
	if len(frozen) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(frozen))
	}
	if frozen[0].Correct != 1 || frozen[0].Total != 1 {
		t.Errorf("frozen standings: got %d/%d, want 1/1 (week 1 only, week 2 excluded)", frozen[0].Correct, frozen[0].Total)
	}
}
