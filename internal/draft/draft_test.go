package draft_test

import (
	"context"
	"testing"
	"time"

	"github.com/evan/football-picks/internal/db/queries"
	"github.com/evan/football-picks/internal/draft"
	"github.com/evan/football-picks/internal/testutil"
)

func TestBuildDraft_Week1NoResults(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(2*time.Hour))
	testutil.SeedGame(t, pool, week.ID, "espn-draft-1", "KC", "DET", "scheduled", nil)

	// GetWeekByNumberAndSeason populates SeasonYear via JOIN.
	w, err := queries.GetWeekByNumberAndSeason(context.Background(), pool, 1, 2025)
	if err != nil {
		t.Fatalf("GetWeekByNumberAndSeason: %v", err)
	}

	d, err := draft.BuildDraft(context.Background(), pool, w)
	if err != nil {
		t.Fatalf("BuildDraft: %v", err)
	}
	if d.Intro == "" {
		t.Error("intro should be non-empty")
	}
	// Week 1 — no previous week results.
	if d.Results != "" {
		t.Errorf("week 1 should have no results section, got %q", d.Results)
	}
	// Games section should mention the seeded game.
	if d.Games == "" {
		t.Error("games section should be non-empty")
	}
}

func TestBuildDraft_Week2IncludesResults(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week1 := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(-2*time.Hour))
	week2 := testutil.SeedWeek(t, pool, season.ID, 2, time.Now().Add(time.Hour))

	winner := "home"
	game1 := testutil.SeedGame(t, pool, week1.ID, "espn-draft-w1", "KC", "DET", "final", &winner)
	testutil.SeedGame(t, pool, week2.ID, "espn-draft-w2", "NE", "MIA", "scheduled", nil)

	u := testutil.SeedUser(t, pool, "uid-draft-1", "DraftUser", "draft@test.com")
	testutil.SeedPick(t, pool, u.ID, game1.ID, "home")

	w2, err := queries.GetWeekByNumberAndSeason(context.Background(), pool, 2, 2025)
	if err != nil {
		t.Fatalf("GetWeekByNumberAndSeason: %v", err)
	}

	d, err := draft.BuildDraft(context.Background(), pool, w2)
	if err != nil {
		t.Fatalf("BuildDraft: %v", err)
	}
	if d.Results == "" {
		t.Error("week 2 should include results from week 1")
	}
}

// TestBuildDraft_ResultsFloorCreditMissedWeek verifies that a user who
// missed the previous week entirely shows up in the email's results
// section with their floor-credited score, not omitted or shown as a
// misleading raw 0 — regression test for the results section having been
// built from raw picks independently of the floor-scoring system.
func TestBuildDraft_ResultsFloorCreditMissedWeek(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	season := testutil.SeedSeason(t, pool, 2025, true)
	week1 := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(-48*time.Hour))
	week2 := testutil.SeedWeek(t, pool, season.ID, 2, time.Now().Add(time.Hour))

	winner := "home"
	game1 := testutil.SeedGameAt(t, pool, week1.ID, "espn-draft-missed-w1", "KC", "DET", "final", &winner, time.Now().Add(-47*time.Hour))
	game2 := testutil.SeedGame(t, pool, week2.ID, "espn-draft-missed-w2", "NE", "MIA", "scheduled", nil)

	alice := testutil.SeedUser(t, pool, "uid-draft-alice", "Alice", "alice-draft@test.com")
	annaB := testutil.SeedUser(t, pool, "uid-draft-annab", "Anna B", "annab-draft@test.com")

	// Alice picks week 1 correctly. Anna B misses week 1 entirely, but picks
	// something in week 2 — making her a season participant eligible for
	// floor credit on the week she missed.
	testutil.SeedPick(t, pool, alice.ID, game1.ID, "home")
	testutil.SeedPick(t, pool, annaB.ID, game2.ID, "home")

	if err := queries.ScorePicks(context.Background(), pool); err != nil {
		t.Fatalf("score picks: %v", err)
	}

	w2, err := queries.GetWeekByNumberAndSeason(context.Background(), pool, 2, 2025)
	if err != nil {
		t.Fatalf("GetWeekByNumberAndSeason: %v", err)
	}

	d, err := draft.BuildDraft(context.Background(), pool, w2)
	if err != nil {
		t.Fatalf("BuildDraft: %v", err)
	}

	if !contains(d.Results, "Alice") {
		t.Errorf("results should mention Alice, got %q", d.Results)
	}
	if !contains(d.Results, "Anna B") {
		t.Errorf("results should mention Anna B even though she missed week 1 entirely (floor-credited), got %q", d.Results)
	}
}

func TestAssemble_JoinsNonEmptySections(t *testing.T) {
	d := &draft.DraftSections{
		Intro:   "Hello!",
		Results: "",
		Records: "Alice 2-0",
		Games:   "KC vs DET",
		Outro:   "Good luck!",
	}
	out := draft.Assemble(d)
	if out == "" {
		t.Fatal("assembled output should not be empty")
	}
	// Empty Results should be omitted.
	for _, s := range []string{"Hello!", "Alice 2-0", "KC vs DET", "Good luck!"} {
		if !contains(out, s) {
			t.Errorf("assembled output missing %q", s)
		}
	}
}

func TestAssemble_AllEmpty(t *testing.T) {
	d := &draft.DraftSections{}
	out := draft.Assemble(d)
	if out != "" {
		t.Errorf("fully empty draft should assemble to empty string, got %q", out)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
