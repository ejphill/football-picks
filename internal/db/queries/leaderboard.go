package queries

import (
	"context"
	"fmt"

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

// GetWeeklyLeaderboardScores returns per-user floor-scored totals.
// Floor scoring credits missed games with the minimum correct count any picker got in that window.
func GetWeeklyLeaderboardScores(ctx context.Context, pool *pgxpool.Pool, weekID int) ([]WeeklyScoreRow, error) {
	rows, err := pool.Query(ctx, `
		WITH included_games AS (
			-- All included games for the week, tagged with their kickoff window.
			SELECT id, date_trunc('hour', kickoff_at) AS kickoff_window
			FROM   games
			WHERE  week_id = $1 AND included_in_picks = TRUE
		),
		window_sizes AS (
			SELECT kickoff_window, COUNT(*) AS game_count
			FROM   included_games
			GROUP  BY kickoff_window
		),
		past_windows AS (
			-- Only windows that have already started.
			SELECT kickoff_window FROM window_sizes WHERE kickoff_window < NOW()
		),
		all_pickers AS (
			-- Every user with at least one pick on an included game this week.
			SELECT DISTINCT p.user_id
			FROM   picks p
			JOIN   included_games ig ON ig.id = p.game_id
		),
		user_window_stats AS (
			-- Per-user, per-window: how many games picked and how many correct.
			SELECT p.user_id,
			       ig.kickoff_window,
			       COUNT(*)                                    AS picks_submitted,
			       COUNT(*) FILTER (WHERE p.is_correct = TRUE) AS correct
			FROM   picks p
			JOIN   included_games ig ON ig.id = p.game_id
			JOIN   past_windows pw   ON pw.kickoff_window = ig.kickoff_window
			GROUP  BY p.user_id, ig.kickoff_window
		),
		window_floors AS (
			-- The floor for each window is the minimum correct count across all
			-- users who submitted at least one pick in that window.
			SELECT kickoff_window, MIN(correct) AS floor
			FROM   user_window_stats
			GROUP  BY kickoff_window
		),
		floor_credits AS (
			-- For every picker × past window, compute how many correct credits
			-- and additional total attempts to award for missed games.
			-- credit = max(0, min(floor - userCorrect, missed))
			SELECT ap.user_id,
			       SUM(
			           GREATEST(0, LEAST(
			               wf.floor - COALESCE(uws.correct, 0),
			               ws.game_count - COALESCE(uws.picks_submitted, 0)
			           ))
			       ) AS credit_correct,
			       SUM(ws.game_count - COALESCE(uws.picks_submitted, 0)) AS credit_total
			FROM       all_pickers ap
			CROSS JOIN past_windows pw
			JOIN       window_sizes   ws  ON ws.kickoff_window  = pw.kickoff_window
			JOIN       window_floors  wf  ON wf.kickoff_window  = pw.kickoff_window
			LEFT JOIN  user_window_stats uws
			               ON uws.user_id = ap.user_id AND uws.kickoff_window = pw.kickoff_window
			GROUP BY ap.user_id
		)
		SELECT u.id,
		       u.display_name,
		       COUNT(*) FILTER (WHERE p.is_correct = TRUE) + COALESCE(fc.credit_correct, 0) AS correct,
		       COUNT(*)                                     + COALESCE(fc.credit_total,   0) AS total
		FROM   picks p
		JOIN   included_games ig ON ig.id = p.game_id
		JOIN   users u           ON u.id  = p.user_id
		LEFT JOIN floor_credits fc ON fc.user_id = p.user_id
		GROUP  BY u.id, u.display_name, fc.credit_correct, fc.credit_total
		ORDER  BY correct DESC, u.display_name
	`, weekID)
	if err != nil {
		return nil, fmt.Errorf("get weekly leaderboard scores: %w", err)
	}
	defer rows.Close()

	var result []WeeklyScoreRow
	for rows.Next() {
		var r WeeklyScoreRow
		if err := rows.Scan(&r.UserID, &r.DisplayName, &r.Correct, &r.Total); err != nil {
			return nil, fmt.Errorf("scan weekly score: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// GetSeasonStandings ranks users by correct DESC, total ASC (fewer picks wins tiebreaker).
// Only included games count.
func GetSeasonStandings(ctx context.Context, pool *pgxpool.Pool, seasonYear int) ([]models.SeasonLeaderboardEntry, error) {
	rows, err := pool.Query(ctx, `
		SELECT u.id, u.display_name,
		       COUNT(*) FILTER (WHERE p.is_correct = TRUE)  AS correct,
		       COUNT(*)                                       AS total
		FROM users u
		JOIN picks p  ON p.user_id = u.id
		JOIN games g  ON p.game_id = g.id
		JOIN weeks w  ON g.week_id = w.id
		JOIN seasons s ON w.season_id = s.id
		WHERE s.year = $1 AND g.included_in_picks = TRUE
		GROUP BY u.id, u.display_name
		ORDER BY correct DESC, total ASC
	`, seasonYear)
	if err != nil {
		return nil, fmt.Errorf("get season standings: %w", err)
	}
	defer rows.Close()

	var entries []models.SeasonLeaderboardEntry
	rank := 1
	for rows.Next() {
		var e models.SeasonLeaderboardEntry
		if err := rows.Scan(&e.UserID, &e.DisplayName, &e.Correct, &e.Total); err != nil {
			return nil, fmt.Errorf("scan standing: %w", err)
		}
		e.Rank = rank
		if e.Total > 0 {
			e.WinPct = float64(e.Correct) / float64(e.Total)
		}
		entries = append(entries, e)
		rank++
	}
	return entries, rows.Err()
}
