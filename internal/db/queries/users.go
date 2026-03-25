package queries

import (
	"context"
	"fmt"

	"github.com/evan/football-picks/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const userColumns = `id, supabase_uid, display_name, email, notify_email, is_admin, created_at, updated_at`

func scanUser(row rowScanner, u *models.User) error {
	return row.Scan(&u.ID, &u.SupabaseUID, &u.DisplayName, &u.Email,
		&u.NotifyEmail, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt)
}

func GetUserBySupabaseUID(ctx context.Context, pool *pgxpool.Pool, supabaseUID string) (*models.User, error) {
	u := &models.User{}
	if err := scanUser(pool.QueryRow(ctx, `
		SELECT `+userColumns+`
		FROM users WHERE supabase_uid = $1
	`, supabaseUID), u); err != nil {
		return nil, fmt.Errorf("get user by supabase_uid: %w", err)
	}
	return u, nil
}

func CreateUser(ctx context.Context, pool *pgxpool.Pool, supabaseUID, displayName, email string) (*models.User, error) {
	u := &models.User{}
	if err := scanUser(pool.QueryRow(ctx, `
		INSERT INTO users (supabase_uid, display_name, email)
		VALUES ($1, $2, $3)
		RETURNING `+userColumns,
		supabaseUID, displayName, email), u); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func UpdateUser(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, displayName string, notifyEmail bool) (*models.User, error) {
	u := &models.User{}
	if err := scanUser(pool.QueryRow(ctx, `
		UPDATE users SET display_name = $2, notify_email = $3, updated_at = NOW()
		WHERE id = $1
		RETURNING `+userColumns,
		id, displayName, notifyEmail), u); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return u, nil
}
