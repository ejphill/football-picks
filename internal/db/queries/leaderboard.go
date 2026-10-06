package queries

import (
	"context"
	"fmt"
	"time"

	"github.com/evan/football-picks/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// floor credits already applied in SQL
type WeeklyScoreRow struct {
	UserID      uuid.UUID
	DisplayName string
	Correct     int
	Total       int
}

// floorScoreCTEs is shared by GetFloorScoredTotals and GetWindowCredits.
// Partitions games into "kickoff windows" (games grouped by truncated
// kickoff hour) per week, so floor credit can be computed per window
// across either a single week or an entire season.
//
// $1 = season year, $2 = week_id (nullable — NULL means "whole season"),
// $3 = exclusive week_number upper bound (nullable — NULL means no cutoff;
// used to compute "standings through the end of last week" for the
// announcement, so it doesn't drift as the current week's own games get
// scored throughout the week).
const floorScoreCTEs = `
	WITH included_games AS (
		SELECT g.id, g.week_id, date_trunc('hour', g.kickoff_at) AS kickoff_window
		FROM   games g
		JOIN   weeks w  ON w.id = g.week_id
		JOIN   seasons s ON s.id = w.season_id
		WHERE  s.year = $1 AND g.included_in_picks = TRUE
		  AND  ($2::int IS NULL OR g.week_id = $2)
		  AND  ($3::int IS NULL OR w.week_number < $3)
	),
	window_sizes AS (
		SELECT week_id, kickoff_window, COUNT(*) AS game_count
		FROM   included_games
		GROUP  BY week_id, kickoff_window
	),
	past_windows AS (
		-- Only windows that have already started.
		SELECT week_id, kickoff_window FROM window_sizes WHERE kickoff_window < NOW()
	),
	season_participants AS (
		-- Every user with at least one pick anywhere THIS SEASON (not just
		-- this week) — this is what lets a fully-missed week still get
		-- floor-credited for an otherwise-active player, while not
		-- fabricating participation for someone who's never played at all.
		SELECT DISTINCT p.user_id
		FROM   picks p
		JOIN   games g   ON g.id = p.game_id
		JOIN   weeks w   ON w.id = g.week_id
		JOIN   seasons s ON s.id = w.season_id
		WHERE  s.year = $1 AND g.included_in_picks = TRUE
	),
	user_window_stats AS (
		-- Per-user, per-week, per-window: how many games picked and how many correct.
		SELECT p.user_id,
		       ig.week_id,
		       ig.kickoff_window,
		       COUNT(*)                                    AS picks_submitted,
		       COUNT(*) FILTER (WHERE p.is_correct = TRUE) AS correct
		FROM   picks p
		JOIN   included_games ig ON ig.id = p.game_id
		JOIN   past_windows pw   ON pw.week_id = ig.week_id AND pw.kickoff_window = ig.kickoff_window
		GROUP  BY p.user_id, ig.week_id, ig.kickoff_window
	),
	window_floors AS (
		-- The floor for each window is the minimum correct count across all
		-- users who submitted at least one pick in that window.
		SELECT week_id, kickoff_window, MIN(correct) AS floor
		FROM   user_window_stats
		GROUP  BY week_id, kickoff_window
	),
	window_credits AS (
		-- For every season participant × past window, compute how many
		-- correct credits and additional total attempts to award for
		-- whatever games they missed in that window.
		-- credit = max(0, min(floor - userCorrect, missed))
		SELECT sp.user_id,
		       pw.week_id,
		       pw.kickoff_window,
		       GREATEST(0, LEAST(
		           wf.floor - COALESCE(uws.correct, 0),
		           ws.game_count - COALESCE(uws.picks_submitted, 0)
		       )) AS credit_correct,
		       ws.game_count - COALESCE(uws.picks_submitted, 0) AS credit_total
		FROM       season_participants sp
		CROSS JOIN past_windows pw
		JOIN       window_sizes  ws ON ws.week_id = pw.week_id AND ws.kickoff_window = pw.kickoff_window
		JOIN       window_floors wf ON wf.week_id = pw.week_id AND wf.kickoff_window = pw.kickoff_window
		LEFT JOIN  user_window_stats uws
		               ON uws.user_id = sp.user_id AND uws.week_id = pw.week_id AND uws.kickoff_window = pw.kickoff_window
	)
`

// GetFloorScoredTotals returns per-user floor-scored totals. If weekID is
// nil, totals are summed across the whole season; otherwise scoped to one
// week. If beforeWeekNumber is set, only weeks strictly before it count —
// used for "standings as of the start of this week," which shouldn't drift
// as the current week's own games get scored. A pick on a game that hasn't
// been scored yet (is_correct still NULL) doesn't count toward total — it's
// neither a win nor a loss until the game is final. A fully- or
// partially-missed kickoff window is credited at the minimum correct count
// any other participant achieved in that window, rather than counting as
// losses.
func GetFloorScoredTotals(ctx context.Context, pool *pgxpool.Pool, seasonYear int, weekID, beforeWeekNumber *int) ([]WeeklyScoreRow, error) {
	rows, err := pool.Query(ctx, floorScoreCTEs+`
		, actual_totals AS (
			SELECT p.user_id,
			       COUNT(*) FILTER (WHERE p.is_correct = TRUE)      AS correct,
			       COUNT(*) FILTER (WHERE p.is_correct IS NOT NULL) AS total
			FROM   picks p
			JOIN   included_games ig ON ig.id = p.game_id
			GROUP  BY p.user_id
		),
		credit_totals AS (
			SELECT user_id,
			       SUM(credit_correct) AS credit_correct,
			       SUM(credit_total)   AS credit_total
			FROM   window_credits
			GROUP  BY user_id
		)
		SELECT sp.user_id,
		       u.display_name,
		       COALESCE(at.correct, 0) + COALESCE(ct.credit_correct, 0) AS correct,
		       COALESCE(at.total,   0) + COALESCE(ct.credit_total,   0) AS total
		FROM      season_participants sp
		JOIN      users u ON u.id = sp.user_id
		LEFT JOIN actual_totals at ON at.user_id = sp.user_id
		LEFT JOIN credit_totals ct ON ct.user_id = sp.user_id
		ORDER BY correct DESC, total ASC, u.display_name
	`, seasonYear, weekID, beforeWeekNumber)
	if err != nil {
		return nil, fmt.Errorf("get floor scored totals: %w", err)
	}
	defer rows.Close()

	var result []WeeklyScoreRow
	for rows.Next() {
		var r WeeklyScoreRow
		if err := rows.Scan(&r.UserID, &r.DisplayName, &r.Correct, &r.Total); err != nil {
			return nil, fmt.Errorf("scan floor scored total: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// GetWeeklyLeaderboardScores is GetFloorScoredTotals scoped to one week.
func GetWeeklyLeaderboardScores(ctx context.Context, pool *pgxpool.Pool, seasonYear, weekID int) ([]WeeklyScoreRow, error) {
	return GetFloorScoredTotals(ctx, pool, seasonYear, &weekID, nil)
}

// GetSeasonStandings ranks users by correct DESC, total ASC (fewer picks
// wins tiebreaker) — GetFloorScoredTotals summed across the whole season,
// live/current (includes the active week's own games as they're scored).
func GetSeasonStandings(ctx context.Context, pool *pgxpool.Pool, seasonYear int) ([]models.SeasonLeaderboardEntry, error) {
	rows, err := GetFloorScoredTotals(ctx, pool, seasonYear, nil, nil)
	return toSeasonStandings(rows, err)
}

// GetSeasonStandingsThroughWeek is GetSeasonStandings frozen as of the start
// of beforeWeekNumber — i.e. it excludes that week and everything after, so
// it doesn't drift as that week's own games get scored throughout the week.
// Used for the announcement's season-records section, which is meant to
// represent "where things stood heading into this week," not a live total.
func GetSeasonStandingsThroughWeek(ctx context.Context, pool *pgxpool.Pool, seasonYear, beforeWeekNumber int) ([]models.SeasonLeaderboardEntry, error) {
	rows, err := GetFloorScoredTotals(ctx, pool, seasonYear, nil, &beforeWeekNumber)
	return toSeasonStandings(rows, err)
}

func toSeasonStandings(rows []WeeklyScoreRow, err error) ([]models.SeasonLeaderboardEntry, error) {
	if err != nil {
		return nil, err
	}

	entries := make([]models.SeasonLeaderboardEntry, 0, len(rows))
	for i, r := range rows {
		e := models.SeasonLeaderboardEntry{
			Rank:        i + 1,
			UserID:      r.UserID,
			DisplayName: r.DisplayName,
			Correct:     r.Correct,
			Total:       r.Total,
		}
		if e.Total > 0 {
			e.WinPct = float64(e.Correct) / float64(e.Total)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// WindowCreditRow is one user's floor credit for one past kickoff window —
// used to decide, per missed game, whether to display it as a credited
// "win" or credited "loss" cell on the weekly leaderboard. Only windows
// where the user has any credit (credit_total > 0) are returned.
type WindowCreditRow struct {
	UserID        uuid.UUID
	KickoffWindow time.Time
	CreditCorrect int
	CreditTotal   int
}

// GetWindowCredits returns per-user, per-window floor credit for one week —
// the finer-grained sibling of GetWeeklyLeaderboardScores's aggregate
// total, needed to assign credit to specific game cells in the UI.
func GetWindowCredits(ctx context.Context, pool *pgxpool.Pool, seasonYear, weekID int) ([]WindowCreditRow, error) {
	rows, err := pool.Query(ctx, floorScoreCTEs+`
		SELECT user_id, kickoff_window, credit_correct, credit_total
		FROM   window_credits
		WHERE  credit_total > 0
	`, seasonYear, weekID, nil)
	if err != nil {
		return nil, fmt.Errorf("get window credits: %w", err)
	}
	defer rows.Close()

	var result []WindowCreditRow
	for rows.Next() {
		var r WindowCreditRow
		if err := rows.Scan(&r.UserID, &r.KickoffWindow, &r.CreditCorrect, &r.CreditTotal); err != nil {
			return nil, fmt.Errorf("scan window credit: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
