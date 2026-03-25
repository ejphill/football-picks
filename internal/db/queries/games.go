package queries

import (
	"context"
	"fmt"

	"github.com/evan/football-picks/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const gameColumns = `id, week_id, espn_game_id, home_team, away_team, home_team_name, away_team_name,
	       spread, kickoff_at, home_score, away_score, winner, status, included_in_picks, created_at, updated_at`

type rowScanner interface {
	Scan(...any) error
}

func scanGame(row rowScanner, g *models.Game) error {
	return row.Scan(
		&g.ID, &g.WeekID, &g.ESPNGameID, &g.HomeTeam, &g.AwayTeam,
		&g.HomeTeamName, &g.AwayTeamName, &g.Spread, &g.KickoffAt,
		&g.HomeScore, &g.AwayScore, &g.Winner, &g.Status, &g.IncludedInPicks,
		&g.CreatedAt, &g.UpdatedAt,
	)
}

// GetGamesByWeek returns only included games for regular users.
func GetGamesByWeek(ctx context.Context, pool *pgxpool.Pool, weekID int) ([]models.Game, error) {
	rows, err := pool.Query(ctx, `
		SELECT `+gameColumns+`
		FROM games
		WHERE week_id = $1 AND included_in_picks = TRUE
		ORDER BY kickoff_at ASC
	`, weekID)
	if err != nil {
		return nil, fmt.Errorf("get games by week: %w", err)
	}
	defer rows.Close()

	var games []models.Game
	for rows.Next() {
		var g models.Game
		if err := scanGame(rows, &g); err != nil {
			return nil, fmt.Errorf("scan game: %w", err)
		}
		games = append(games, g)
	}
	return games, rows.Err()
}

// GetAllGamesByWeek returns all games regardless of included_in_picks — admin use.
func GetAllGamesByWeek(ctx context.Context, pool *pgxpool.Pool, weekID int) ([]models.Game, error) {
	rows, err := pool.Query(ctx, `
		SELECT `+gameColumns+`
		FROM games
		WHERE week_id = $1
		ORDER BY kickoff_at ASC
	`, weekID)
	if err != nil {
		return nil, fmt.Errorf("get all games by week: %w", err)
	}
	defer rows.Close()

	var games []models.Game
	for rows.Next() {
		var g models.Game
		if err := scanGame(rows, &g); err != nil {
			return nil, fmt.Errorf("scan game: %w", err)
		}
		games = append(games, g)
	}
	return games, rows.Err()
}

func GetGameByID(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (*models.Game, error) {
	g := &models.Game{}
	err := scanGame(pool.QueryRow(ctx, `
		SELECT `+gameColumns+`
		FROM games WHERE id = $1
	`, id), g)
	if err != nil {
		return nil, fmt.Errorf("get game by id: %w", err)
	}
	return g, nil
}

// UpsertGame inserts or updates a game row keyed on espn_game_id.
// On insert, included_in_picks defaults to FALSE for Thursday/Friday games (DOW 4/5), TRUE otherwise.
// On update, included_in_picks is left alone to preserve admin overrides.
func UpsertGame(ctx context.Context, pool *pgxpool.Pool, g *models.Game) (*models.Game, error) {
	result := &models.Game{}
	err := scanGame(pool.QueryRow(ctx, `
		INSERT INTO games (week_id, espn_game_id, home_team, away_team, home_team_name, away_team_name,
		                   spread, kickoff_at, home_score, away_score, winner, status, included_in_picks)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
		        EXTRACT(DOW FROM $8 AT TIME ZONE 'America/New_York') NOT IN (4, 5))
		ON CONFLICT (espn_game_id) DO UPDATE SET
		    home_team      = EXCLUDED.home_team,
		    away_team      = EXCLUDED.away_team,
		    home_team_name = EXCLUDED.home_team_name,
		    away_team_name = EXCLUDED.away_team_name,
		    spread         = EXCLUDED.spread,
		    kickoff_at     = EXCLUDED.kickoff_at,
		    home_score     = EXCLUDED.home_score,
		    away_score     = EXCLUDED.away_score,
		    winner         = EXCLUDED.winner,
		    status         = EXCLUDED.status,
		    updated_at     = NOW()
		RETURNING `+gameColumns,
		g.WeekID, g.ESPNGameID, g.HomeTeam, g.AwayTeam, g.HomeTeamName, g.AwayTeamName,
		g.Spread, g.KickoffAt, g.HomeScore, g.AwayScore, g.Winner, g.Status,
	), result)
	if err != nil {
		return nil, fmt.Errorf("upsert game: %w", err)
	}
	return result, nil
}

func SetGameIncluded(ctx context.Context, pool *pgxpool.Pool, gameID uuid.UUID, included bool) (*models.Game, error) {
	result := &models.Game{}
	err := scanGame(pool.QueryRow(ctx, `
		UPDATE games SET included_in_picks = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING `+gameColumns,
		gameID, included,
	), result)
	if err != nil {
		return nil, fmt.Errorf("set game included: %w", err)
	}
	return result, nil
}

// ScorePicks re-scores all picks for final games — idempotent, safe to re-run.
func ScorePicks(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		UPDATE picks
		SET is_correct = (picks.picked_team = games.winner),
		    updated_at = NOW()
		FROM games
		WHERE picks.game_id = games.id
		  AND games.status = 'final'
		  AND games.winner IS NOT NULL
		  AND (picks.is_correct IS NULL OR picks.updated_at < games.updated_at)
	`)
	if err != nil {
		return fmt.Errorf("score picks: %w", err)
	}
	return nil
}

func GetWeekByID(ctx context.Context, pool *pgxpool.Pool, id int) (*models.Week, error) {
	w := &models.Week{}
	err := pool.QueryRow(ctx, `
		SELECT w.id, w.season_id, s.year, w.week_number, w.picks_lock_at
		FROM weeks w
		JOIN seasons s ON w.season_id = s.id
		WHERE w.id = $1
	`, id).Scan(&w.ID, &w.SeasonID, &w.SeasonYear, &w.WeekNumber, &w.PicksLockAt)
	if err != nil {
		return nil, fmt.Errorf("get week by id: %w", err)
	}
	return w, nil
}

func GetWeekByNumberAndSeason(ctx context.Context, pool *pgxpool.Pool, weekNumber, seasonYear int) (*models.Week, error) {
	w := &models.Week{}
	err := pool.QueryRow(ctx, `
		SELECT w.id, w.season_id, s.year, w.week_number, w.picks_lock_at
		FROM weeks w
		JOIN seasons s ON w.season_id = s.id
		WHERE w.week_number = $1 AND s.year = $2
	`, weekNumber, seasonYear).Scan(&w.ID, &w.SeasonID, &w.SeasonYear, &w.WeekNumber, &w.PicksLockAt)
	if err != nil {
		return nil, fmt.Errorf("get week by number and season: %w", err)
	}
	return w, nil
}

func GetActiveWeek(ctx context.Context, pool *pgxpool.Pool) (*models.Week, error) {
	w := &models.Week{}
	err := pool.QueryRow(ctx, `
		SELECT w.id, w.season_id, s.year, w.week_number, w.picks_lock_at
		FROM weeks w
		JOIN seasons s ON w.season_id = s.id
		WHERE s.is_active = TRUE
		ORDER BY
		    CASE WHEN w.picks_lock_at >= NOW() THEN 0 ELSE 1 END,
		    CASE WHEN w.picks_lock_at >= NOW() THEN w.picks_lock_at END ASC NULLS LAST,
		    w.week_number DESC
		LIMIT 1
	`).Scan(&w.ID, &w.SeasonID, &w.SeasonYear, &w.WeekNumber, &w.PicksLockAt)
	if err != nil {
		return nil, fmt.Errorf("get active week: %w", err)
	}
	return w, nil
}
