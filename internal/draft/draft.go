package draft

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/evan/football-picks/internal/db/queries"
	"github.com/evan/football-picks/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ordinals = []string{
	"first", "second", "third", "fourth", "fifth",
	"sixth", "seventh", "eighth", "ninth", "tenth",
}

var dayOrder = []string{"Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday", "Monday"}

// DraftSections holds the six editable sections of an announcement.
type DraftSections struct {
	Intro        string `json:"intro"`
	Results      string `json:"results"`
	Records      string `json:"records"`
	PreGamesNote string `json:"pre_games_note"`
	Games        string `json:"games"`
	Outro        string `json:"outro"`
}

// AnnounceTarget computes the scheduler's automatic-announcement target:
// 1pm ET on the Saturday of the given week, based on its earliest kickoff.
func AnnounceTarget(ctx context.Context, pool *pgxpool.Pool, weekID int) (time.Time, error) {
	games, err := queries.GetGamesByWeek(ctx, pool, weekID)
	if err != nil {
		return time.Time{}, err
	}
	if len(games) == 0 {
		return time.Time{}, fmt.Errorf("no games for week %d", weekID)
	}

	est, _ := time.LoadLocation("America/New_York")
	first := games[0].KickoffAt.In(est)
	daysUntilSat := (int(time.Saturday) - int(first.Weekday()) + 7) % 7
	sat := first.AddDate(0, 0, daysUntilSat)
	return time.Date(sat.Year(), sat.Month(), sat.Day(), 13, 0, 0, 0, est), nil
}

// BuildDraft assembles a default DraftSections for the given week.
func BuildDraft(ctx context.Context, pool *pgxpool.Pool, week *models.Week) (*DraftSections, error) {
	d := &DraftSections{
		Intro: "Hello everybody!!",
		Outro: "Good luck to everyone this week!\n\n-Jack",
	}

	// Games for this week.
	games, err := queries.GetGamesByWeek(ctx, pool, week.ID)
	if err != nil {
		return nil, fmt.Errorf("get games: %w", err)
	}
	d.Games = buildGames(games)

	// Season standings for records section — frozen as of the start of this
	// week, computed once and cached on the week row thereafter. Deliberately
	// a permanent cache, not a TTL one: the value represents "standings as
	// of the start of this week," which shouldn't change as the week
	// progresses, so there's no reason to ever recompute it once set (the
	// tradeoff: a rare after-the-fact score correction for a past week
	// wouldn't be reflected — acceptable for a recap display, since the
	// real leaderboard stays fully live regardless).
	frozen, err := queries.GetFrozenRecords(ctx, pool, week.ID)
	if err != nil {
		return nil, fmt.Errorf("get frozen records: %w", err)
	}
	if frozen != nil {
		d.Records = *frozen
	} else {
		standings, err := queries.GetSeasonStandingsThroughWeek(ctx, pool, week.SeasonYear, week.WeekNumber)
		if err != nil {
			return nil, fmt.Errorf("get standings: %w", err)
		}
		d.Records = buildRecords(standings)
		if err := queries.SetFrozenRecords(ctx, pool, week.ID, d.Records); err != nil {
			// Non-fatal — the records were computed fine, just couldn't be
			// cached for next time, so the next call recomputes instead.
			slog.Warn("draft: failed to cache frozen records", "week_id", week.ID, "err", err)
		}
	}

	// Previous week results (omit for week 1).
	if week.WeekNumber > 1 {
		prevWeek, err := queries.GetWeekByNumberAndSeason(ctx, pool, week.WeekNumber-1, week.SeasonYear)
		if err != nil {
			return nil, fmt.Errorf("get prev week: %w", err)
		}
		// Floor-scored, same as the weekly leaderboard — a player who missed
		// a window gets credited rather than showing up at a misleading raw
		// (or absent) 0.
		prevScores, err := queries.GetWeeklyLeaderboardScores(ctx, pool, week.SeasonYear, prevWeek.ID)
		if err != nil {
			return nil, fmt.Errorf("get prev scores: %w", err)
		}
		d.Results = buildResults(prevScores)
	}

	return d, nil
}

// Assemble joins non-empty sections into the full announcement body.
func Assemble(d *DraftSections) string {
	parts := []string{d.Intro, d.Results, d.Records, d.PreGamesNote, d.Games, d.Outro}
	var nonEmpty []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			nonEmpty = append(nonEmpty, strings.TrimSpace(p))
		}
	}
	return strings.Join(nonEmpty, "\n\n")
}

func buildResults(scores []queries.WeeklyScoreRow) string {
	if len(scores) == 0 {
		return ""
	}

	// Every season participant's Total should match the week's full game
	// count once all its windows are past (which they are, by the time this
	// runs for the *previous* week) — but take the max defensively rather
	// than assuming they're all identical.
	totalGames := 0
	for _, s := range scores {
		if s.Total > totalGames {
			totalGames = s.Total
		}
	}

	// Group users by (floor-scored) correct count.
	scoreMap := map[int][]string{}
	for _, s := range scores {
		scoreMap[s.Correct] = append(scoreMap[s.Correct], s.DisplayName)
	}

	distinctScores := make([]int, 0, len(scoreMap))
	for s := range scoreMap {
		distinctScores = append(distinctScores, s)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(distinctScores)))

	numGroups := len(distinctScores)
	lines := []string{"Here are the results from last week:\n"}
	for i, score := range distinctScores {
		names := scoreMap[score]
		sort.Strings(names)
		bangs := strings.Repeat("!", numGroups-i)
		isLast := i == numGroups-1 && numGroups > 1

		var label string
		switch {
		case isLast:
			label = "And in last"
		case i < len(ordinals):
			label = "In " + ordinals[i]
		default:
			label = fmt.Sprintf("In %dth", i+1)
		}

		verb := "were"
		if len(names) == 1 {
			verb = "was"
		}

		lines = append(lines, fmt.Sprintf(
			"%s, with %d out of the %d games right %s\u2026\u2026.%s%s",
			label, score, totalGames, verb, formatNames(names), bangs,
		))
	}
	return strings.Join(lines, "\n")
}

func buildRecords(standings []models.SeasonLeaderboardEntry) string {
	if len(standings) == 0 {
		return ""
	}

	type group struct {
		key   string
		names []string
	}
	var order []string
	groupMap := map[string]*group{}
	for _, e := range standings {
		losses := e.Total - e.Correct
		key := fmt.Sprintf("%d-%d", e.Correct, losses)
		if _, ok := groupMap[key]; !ok {
			groupMap[key] = &group{key: key}
			order = append(order, key)
		}
		groupMap[key].names = append(groupMap[key].names, e.DisplayName)
	}

	lines := []string{"Here are the total records thus far:\n"}
	for _, key := range order {
		g := groupMap[key]
		lines = append(lines, fmt.Sprintf("%s **(%s)**", formatNames(g.names), g.key))
	}
	return strings.Join(lines, "\n")
}

func buildGames(games []models.Game) string {
	if len(games) == 0 {
		return ""
	}

	est, _ := time.LoadLocation("America/New_York")
	dayMap := map[string][]models.Game{}
	for _, g := range games {
		day := g.KickoffAt.In(est).Weekday().String()
		dayMap[day] = append(dayMap[day], g)
	}

	lines := []string{"Here are the games for this week:\n"}
	for _, day := range dayOrder {
		gs, ok := dayMap[day]
		if !ok {
			continue
		}
		lines = append(lines, day+":")
		for _, g := range gs {
			lines = append(lines, fmt.Sprintf("%s vs. %s", g.AwayTeamName, g.HomeTeamName))
		}
	}
	return strings.Join(lines, "\n")
}

func formatNames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	default:
		return strings.Join(names[:len(names)-1], ", ") + ", and " + names[len(names)-1]
	}
}
