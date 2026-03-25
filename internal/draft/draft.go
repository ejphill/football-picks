package draft

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/evan/football-picks/internal/db/queries"
	"github.com/evan/football-picks/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ordinals = []string{
	"first", "second", "third", "fourth", "fifth",
	"sixth", "seventh", "eighth", "ninth", "tenth",
}

var dayOrder = []string{"Thursday", "Friday", "Saturday", "Sunday", "Monday", "Tuesday", "Wednesday"}

// DraftSections holds the six editable sections of an announcement.
type DraftSections struct {
	Intro        string `json:"intro"`
	Results      string `json:"results"`
	Records      string `json:"records"`
	PreGamesNote string `json:"pre_games_note"`
	Games        string `json:"games"`
	Outro        string `json:"outro"`
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

	// Season standings for records section.
	standings, err := queries.GetSeasonStandings(ctx, pool, week.SeasonYear)
	if err != nil {
		return nil, fmt.Errorf("get standings: %w", err)
	}
	d.Records = buildRecords(standings)

	// Previous week results (omit for week 1).
	if week.WeekNumber > 1 {
		prevWeek, err := queries.GetWeekByNumberAndSeason(ctx, pool, week.WeekNumber-1, week.SeasonYear)
		if err != nil {
			return nil, fmt.Errorf("get prev week: %w", err)
		}
		allPicks, err := queries.GetAllPicksByWeek(ctx, pool, prevWeek.ID)
		if err != nil {
			return nil, fmt.Errorf("get prev picks: %w", err)
		}
		prevGames, err := queries.GetGamesByWeek(ctx, pool, prevWeek.ID)
		if err != nil {
			return nil, fmt.Errorf("get prev games: %w", err)
		}
		d.Results = buildResults(allPicks, prevGames)
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

func buildResults(allPicks []queries.UserPickRow, games []models.Game) string {
	if len(allPicks) == 0 {
		return ""
	}

	totalGames := len(games)

	// Aggregate correct count per user.
	userCorrect := map[uuid.UUID]int{}
	userNames := map[uuid.UUID]string{}
	for _, p := range allPicks {
		userNames[p.UserID] = p.DisplayName
		if p.IsCorrect != nil && *p.IsCorrect {
			userCorrect[p.UserID]++
		}
	}

	// Group users by correct count.
	scoreMap := map[int][]string{}
	for uid, name := range userNames {
		scoreMap[userCorrect[uid]] = append(scoreMap[userCorrect[uid]], name)
	}

	scores := make([]int, 0, len(scoreMap))
	for s := range scoreMap {
		scores = append(scores, s)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(scores)))

	numGroups := len(scores)
	lines := []string{"Here are the results from last week:\n"}
	for i, score := range scores {
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
