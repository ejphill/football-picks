package espn

import (
	"testing"
)

func TestParseESPNEvent(t *testing.T) {
	homeScore := 24
	awayScore := 17

	event := Event{
		ID:   "401547417",
		Date: "2025-09-07T17:00:00Z",
		Competitions: []Competition{
			{
				Competitors: []Competitor{
					{HomeAway: "home", Team: Team{Abbreviation: "KC", DisplayName: "Kansas City Chiefs"}, Score: "24"},
					{HomeAway: "away", Team: Team{Abbreviation: "DET", DisplayName: "Detroit Lions"}, Score: "17"},
				},
				Odds: []Odds{{Details: "KC -3.5"}},
				Status: EventStatus{Type: StatusType{Name: "STATUS_FINAL", Completed: true}},
			},
		},
	}

	g, err := parseEvent(event, 1)
	if err != nil {
		t.Fatalf("parseEvent error: %v", err)
	}

	if g.ESPNGameID != "401547417" {
		t.Errorf("ESPNGameID: got %q, want %q", g.ESPNGameID, "401547417")
	}
	if g.HomeTeam != "KC" {
		t.Errorf("HomeTeam: got %q, want %q", g.HomeTeam, "KC")
	}
	if g.AwayTeam != "DET" {
		t.Errorf("AwayTeam: got %q, want %q", g.AwayTeam, "DET")
	}
	if g.HomeScore == nil || *g.HomeScore != homeScore {
		t.Errorf("HomeScore: got %v, want %d", g.HomeScore, homeScore)
	}
	if g.AwayScore == nil || *g.AwayScore != awayScore {
		t.Errorf("AwayScore: got %v, want %d", g.AwayScore, awayScore)
	}
	if g.Status != "final" {
		t.Errorf("Status: got %q, want %q", g.Status, "final")
	}
	if g.Winner == nil || *g.Winner != "home" {
		t.Errorf("Winner: got %v, want %q", g.Winner, "home")
	}
	if g.Spread == nil || *g.Spread != -3.5 {
		t.Errorf("Spread: got %v, want -3.5", g.Spread)
	}
}

func TestDetermineWinner(t *testing.T) {
	cases := []struct {
		home, away int
		want       string
	}{
		{24, 17, "home"},
		{10, 21, "away"},
		{14, 14, "tie"},
	}
	for _, tc := range cases {
		got := determineWinner(tc.home, tc.away)
		if got != tc.want {
			t.Errorf("determineWinner(%d, %d) = %q, want %q", tc.home, tc.away, got, tc.want)
		}
	}
}

func TestParseSpread(t *testing.T) {
	cases := []struct {
		details string
		want    *float64
	}{
		{"KC -3.5", ptr(-3.5)},
		{"NE +7", ptr(7.0)},
		{"EVEN", nil},
		{"PHI -6", ptr(-6.0)},
		{"", nil},
	}
	for _, tc := range cases {
		got := parseSpread(tc.details)
		if tc.want == nil {
			if got != nil {
				t.Errorf("parseSpread(%q) = %v, want nil", tc.details, *got)
			}
		} else {
			if got == nil || *got != *tc.want {
				t.Errorf("parseSpread(%q) = %v, want %v", tc.details, got, *tc.want)
			}
		}
	}
}

func TestMapStatus(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"STATUS_FINAL", "final"},
		{"STATUS_FINAL_OVERTIME", "final"},
		{"STATUS_IN_PROGRESS", "in_progress"},
		{"STATUS_HALFTIME", "in_progress"},
		{"STATUS_SCHEDULED", "scheduled"},
		{"", "scheduled"},
	}
	for _, tc := range cases {
		got := mapStatus(tc.input)
		if got != tc.want {
			t.Errorf("mapStatus(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func ptr(v float64) *float64 { return &v }

func TestNewClient(t *testing.T) {
	c := NewClient()
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.http == nil {
		t.Fatal("Client.http is nil")
	}
}
