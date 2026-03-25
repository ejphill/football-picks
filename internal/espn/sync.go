package espn

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/evan/football-picks/internal/db/queries"
	"github.com/evan/football-picks/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Syncer struct {
	client   *Client
	pool     *pgxpool.Pool
	onScored func() // called after ScorePicks completes successfully; may be nil
}

func NewSyncer(pool *pgxpool.Pool) *Syncer {
	return &Syncer{client: NewClient(), pool: pool}
}

// SetOnScored registers a hook for after ScorePicks succeeds — pass LeaderboardCache.InvalidateScores.
func (s *Syncer) SetOnScored(fn func()) {
	s.onScored = fn
}

// SyncWeek upserts games from ESPN and scores picks if any game became final.
func (s *Syncer) SyncWeek(ctx context.Context, week *models.Week, seasonType int) error {
	sb, err := s.client.FetchWeek(week.WeekNumber, week.SeasonYear, seasonType)
	if err != nil {
		return fmt.Errorf("fetch week: %w", err)
	}

	anyFinal := false
	for _, event := range sb.Events {
		g, err := parseEvent(event, week.ID)
		if err != nil {
			slog.Warn("espn: skip event", "event_id", event.ID, "err", err)
			continue
		}

		result, err := queries.UpsertGame(ctx, s.pool, g)
		if err != nil {
			slog.Error("espn: upsert game", "event_id", event.ID, "err", err)
			continue
		}
		if result.Status == "final" {
			anyFinal = true
		}
	}

	if anyFinal {
		if err := queries.ScorePicks(ctx, s.pool); err != nil {
			slog.Error("espn: score picks after sync", "err", err)
		} else if s.onScored != nil {
			s.onScored()
		}
	}

	return nil
}

func parseEvent(event Event, weekID int) (*models.Game, error) {
	if len(event.Competitions) == 0 {
		return nil, fmt.Errorf("no competitions")
	}
	comp := event.Competitions[0]

	g := &models.Game{
		WeekID:     weekID,
		ESPNGameID: event.ID,
	}

	// ESPN omits seconds in some responses, so try both layouts
	var t time.Time
	var err error
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04Z"} {
		t, err = time.Parse(layout, event.Date)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("parse date %q: %w", event.Date, err)
	}
	g.KickoffAt = t

	for _, c := range comp.Competitors {
		if c.HomeAway == "home" {
			g.HomeTeam = c.Team.Abbreviation
			g.HomeTeamName = c.Team.DisplayName
			if score, err := strconv.Atoi(c.Score); err == nil {
				g.HomeScore = &score
			}
		} else {
			g.AwayTeam = c.Team.Abbreviation
			g.AwayTeamName = c.Team.DisplayName
			if score, err := strconv.Atoi(c.Score); err == nil {
				g.AwayScore = &score
			}
		}
	}

	g.Status = mapStatus(comp.Status.Type.Name)

	if comp.Status.Type.Completed && g.HomeScore != nil && g.AwayScore != nil {
		w := determineWinner(*g.HomeScore, *g.AwayScore)
		g.Winner = &w
	}

	if len(comp.Odds) > 0 {
		g.Spread = parseSpread(comp.Odds[0].Details)
	}

	return g, nil
}

func mapStatus(espnStatus string) string {
	switch espnStatus {
	case "STATUS_FINAL", "STATUS_FINAL_OVERTIME":
		return "final"
	case "STATUS_IN_PROGRESS", "STATUS_HALFTIME":
		return "in_progress"
	default:
		return "scheduled"
	}
}

func determineWinner(home, away int) string {
	switch {
	case home > away:
		return "home"
	case away > home:
		return "away"
	default:
		return "tie"
	}
}

// parseSpread extracts the numeric spread from strings like "KC -3.5" or "NE +7".
func parseSpread(details string) *float64 {
	parts := strings.Fields(details)
	for _, p := range parts {
		if len(p) > 0 && (p[0] == '-' || p[0] == '+' || (p[0] >= '0' && p[0] <= '9')) {
			if v, err := strconv.ParseFloat(p, 64); err == nil {
				return &v
			}
		}
	}
	return nil
}
