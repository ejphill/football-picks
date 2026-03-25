package queries

import (
	"context"
	"errors"
	"fmt"

	"github.com/evan/football-picks/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const announcementColumns = `id, author_id, week_id, intro, body_json, published_at, created_at`

func scanAnnouncement(row rowScanner, a *models.Announcement) error {
	return row.Scan(&a.ID, &a.AuthorID, &a.WeekID, &a.Intro, &a.BodyJSON, &a.PublishedAt, &a.CreatedAt)
}

func CreateAnnouncement(ctx context.Context, pool *pgxpool.Pool, authorID uuid.UUID, weekID int, intro string) (*models.Announcement, error) {
	a := &models.Announcement{}
	if err := scanAnnouncement(pool.QueryRow(ctx, `
		INSERT INTO announcements (author_id, week_id, intro)
		VALUES ($1, $2, $3)
		RETURNING `+announcementColumns,
		authorID, weekID, intro), a); err != nil {
		return nil, fmt.Errorf("create announcement: %w", err)
	}
	return a, nil
}

func GetAnnouncementByID(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (*models.Announcement, error) {
	a := &models.Announcement{}
	if err := scanAnnouncement(pool.QueryRow(ctx, `
		SELECT `+announcementColumns+`
		FROM announcements WHERE id = $1
	`, id), a); err != nil {
		return nil, fmt.Errorf("get announcement: %w", err)
	}
	return a, nil
}

func GetAnnouncementsBySeason(ctx context.Context, pool *pgxpool.Pool, seasonYear int) ([]models.Announcement, error) {
	rows, err := pool.Query(ctx, `
		SELECT `+announcementColumns+`
		FROM announcements
		WHERE week_id IN (
			SELECT w.id FROM weeks w
			JOIN seasons s ON w.season_id = s.id
			WHERE s.year = $1
		)
		ORDER BY published_at DESC
	`, seasonYear)
	if err != nil {
		return nil, fmt.Errorf("get announcements by season: %w", err)
	}
	defer rows.Close()

	var result []models.Announcement
	for rows.Next() {
		var a models.Announcement
		if err := scanAnnouncement(rows, &a); err != nil {
			return nil, fmt.Errorf("scan announcement: %w", err)
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

func GetAnnouncementsByWeek(ctx context.Context, pool *pgxpool.Pool, weekID int) ([]models.Announcement, error) {
	rows, err := pool.Query(ctx, `
		SELECT `+announcementColumns+`
		FROM announcements WHERE week_id = $1
		ORDER BY published_at DESC
	`, weekID)
	if err != nil {
		return nil, fmt.Errorf("get announcements by week: %w", err)
	}
	defer rows.Close()

	var result []models.Announcement
	for rows.Next() {
		var a models.Announcement
		if err := scanAnnouncement(rows, &a); err != nil {
			return nil, fmt.Errorf("scan announcement: %w", err)
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

// CreateAutoAnnouncement inserts the scheduler's weekly announcement.
// Returns nil if another instance beat it to the insert.
func CreateAutoAnnouncement(ctx context.Context, pool *pgxpool.Pool, authorID uuid.UUID, weekID int, intro string) (*models.Announcement, error) {
	a := &models.Announcement{}
	err := scanAnnouncement(pool.QueryRow(ctx, `
		INSERT INTO announcements (author_id, week_id, intro)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
		RETURNING `+announcementColumns,
		authorID, weekID, intro), a)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // another instance won the race
		}
		return nil, fmt.Errorf("create auto announcement: %w", err)
	}
	return a, nil
}

// GetUsersForNotification returns subscribed users who haven't been successfully notified for weekID.
func GetUsersForNotification(ctx context.Context, pool *pgxpool.Pool, weekID int) ([]models.User, error) {
	rows, err := pool.Query(ctx, `
		SELECT `+userColumns+`
		FROM users
		WHERE notify_email = TRUE
		  AND NOT EXISTS (
			  SELECT 1 FROM notification_log
			  WHERE  user_id = users.id
			    AND  week_id = $1
			    AND  success = TRUE
		  )
	`, weekID)
	if err != nil {
		return nil, fmt.Errorf("get users for notification: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := scanUser(rows, &u); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func LogNotification(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, weekID int, success bool, errMsg string) error {
	var errPtr *string
	if errMsg != "" {
		errPtr = &errMsg
	}
	// partial unique index only covers success=TRUE rows
	_, err := pool.Exec(ctx, `
		INSERT INTO notification_log (user_id, week_id, success, error_msg)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT DO NOTHING
	`, userID, weekID, success, errPtr)
	if err != nil {
		return fmt.Errorf("log notification: %w", err)
	}
	return nil
}
