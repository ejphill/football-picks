package queries

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/evan/football-picks/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrPickNotFound = errors.New("pick not found")

// lock check is inside the query, so there's no TOCTOU window
var ErrPickLocked = errors.New("pick locked: game has kicked off")

func GetPicksByUserAndWeek(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, weekID int) ([]models.Pick, error) {
	rows, err := pool.Query(ctx, `
		SELECT p.id, p.user_id, p.game_id, p.picked_team, p.is_correct, p.created_at, p.updated_at
		FROM picks p
		JOIN games g ON p.game_id = g.id
		WHERE p.user_id = $1 AND g.week_id = $2
		ORDER BY g.kickoff_at ASC
	`, userID, weekID)
	if err != nil {
		return nil, fmt.Errorf("get picks by user and week: %w", err)
	}
	defer rows.Close()

	var picks []models.Pick
	for rows.Next() {
		var p models.Pick
		if err := rows.Scan(&p.ID, &p.UserID, &p.GameID, &p.PickedTeam, &p.IsCorrect, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan pick: %w", err)
		}
		picks = append(picks, p)
	}
	return picks, rows.Err()
}

// Lock is enforced atomically: kicked-off game → INSERT finds no row → ErrNoRows.
// The follow-up EXISTS check then distinguishes "game not found" vs ErrPickLocked.
func UpsertPick(ctx context.Context, pool *pgxpool.Pool, userID, gameID uuid.UUID, pickedTeam string) (*models.Pick, bool, error) {
	p := &models.Pick{}
	var created bool
	err := pool.QueryRow(ctx, `
		INSERT INTO picks (user_id, game_id, picked_team, is_correct)
		SELECT $1, $2, $3,
		       CASE WHEN g.status = 'final' THEN $3::text = g.winner ELSE NULL END
		FROM games g
		WHERE g.id = $2
		  AND g.kickoff_at > NOW()
		ON CONFLICT (user_id, game_id) DO UPDATE SET
		    picked_team = EXCLUDED.picked_team,
		    is_correct  = EXCLUDED.is_correct,
		    updated_at  = NOW()
		RETURNING id, user_id, game_id, picked_team, is_correct, created_at, updated_at, (xmax = 0)
	`, userID, gameID, pickedTeam).Scan(
		&p.ID, &p.UserID, &p.GameID, &p.PickedTeam, &p.IsCorrect, &p.CreatedAt, &p.UpdatedAt, &created,
	)
	if err == nil {
		return p, created, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, fmt.Errorf("upsert pick: %w", err)
	}

	// ErrNoRows means either the game doesn't exist or it has kicked off.
	// Check on the error path so the happy path stays a single query.
	var exists bool
	_ = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM games WHERE id = $1)`, gameID).Scan(&exists)
	if !exists {
		return nil, false, pgx.ErrNoRows
	}
	return nil, false, ErrPickLocked
}

func DeletePick(ctx context.Context, pool *pgxpool.Pool, userID, gameID uuid.UUID) error {
	// Enforce the lock atomically: only delete if the game has not yet kicked off.
	tag, err := pool.Exec(ctx, `
		DELETE FROM picks
		USING games g
		WHERE picks.user_id = $1
		  AND picks.game_id = $2
		  AND g.id = $2
		  AND g.kickoff_at > NOW()
	`, userID, gameID)
	if err != nil {
		return fmt.Errorf("delete pick: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}

	// 0 rows affected — game not found, game locked, or pick not found.
	// Fetch kickoff_at to distinguish the cases.
	var kickoffAt pgtype.Timestamptz
	err = pool.QueryRow(ctx, `SELECT kickoff_at FROM games WHERE id = $1`, gameID).Scan(&kickoffAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPickNotFound
	}
	if kickoffAt.Valid && kickoffAt.Time.Before(time.Now()) {
		return ErrPickLocked
	}
	return ErrPickNotFound
}

type UserPickRow struct {
	UserID      uuid.UUID
	DisplayName string
	GameID      uuid.UUID
	PickedTeam  string
	IsCorrect   *bool
}

// GetAllPicksByWeek returns picks for all users on included games — used by the draft builder.
func GetAllPicksByWeek(ctx context.Context, pool *pgxpool.Pool, weekID int) ([]UserPickRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT u.id, u.display_name, p.game_id, p.picked_team, p.is_correct
		FROM picks p
		JOIN users u ON p.user_id = u.id
		JOIN games g ON p.game_id = g.id
		WHERE g.week_id = $1 AND g.included_in_picks = TRUE
		ORDER BY u.display_name, g.kickoff_at
	`, weekID)
	if err != nil {
		return nil, fmt.Errorf("get all picks by week: %w", err)
	}
	defer rows.Close()

	var result []UserPickRow
	for rows.Next() {
		var r UserPickRow
		if err := rows.Scan(&r.UserID, &r.DisplayName, &r.GameID, &r.PickedTeam, &r.IsCorrect); err != nil {
			return nil, fmt.Errorf("scan pick row: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// GetPicksForUsersAndWeek is like GetAllPicksByWeek but scoped to a set of users.
func GetPicksForUsersAndWeek(ctx context.Context, pool *pgxpool.Pool, weekID int, userIDs []uuid.UUID) ([]UserPickRow, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	ids := make([]string, len(userIDs))
	for i, id := range userIDs {
		ids[i] = id.String()
	}
	rows, err := pool.Query(ctx, `
		SELECT u.id, u.display_name, p.game_id, p.picked_team, p.is_correct
		FROM picks p
		JOIN users u ON p.user_id = u.id
		JOIN games g ON p.game_id = g.id
		WHERE g.week_id = $1
		  AND g.included_in_picks = TRUE
		  AND u.id = ANY($2::uuid[])
		ORDER BY u.display_name, g.kickoff_at
	`, weekID, ids)
	if err != nil {
		return nil, fmt.Errorf("get picks for users and week: %w", err)
	}
	defer rows.Close()

	var result []UserPickRow
	for rows.Next() {
		var r UserPickRow
		if err := rows.Scan(&r.UserID, &r.DisplayName, &r.GameID, &r.PickedTeam, &r.IsCorrect); err != nil {
			return nil, fmt.Errorf("scan pick row: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
