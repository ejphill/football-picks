package queries

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FailedNotificationRow struct {
	UserID              uuid.UUID
	WeekID              int
	WeekNumber          int
	DisplayName         string
	Email               string
	AnnouncementID      uuid.UUID
	AnnouncementIntro   string
	Attempts            int
}

// GetFailedNotifications returns rows eligible for retry.
func GetFailedNotifications(ctx context.Context, pool *pgxpool.Pool) ([]FailedNotificationRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT
			nl.user_id,
			nl.week_id,
			w.week_number,
			u.display_name,
			u.email,
			a.id              AS announcement_id,
			a.intro           AS announcement_intro,
			COUNT(*)::int     AS attempts
		FROM notification_log nl
		JOIN users  u ON u.id = nl.user_id AND u.notify_email = TRUE
		JOIN weeks  w ON w.id = nl.week_id
		JOIN LATERAL (
			SELECT id, intro
			FROM   announcements
			WHERE  week_id = nl.week_id
			ORDER  BY published_at DESC
			LIMIT  1
		) a ON TRUE
		WHERE nl.success = FALSE
		GROUP BY nl.user_id, nl.week_id, w.week_number,
		         u.display_name, u.email, a.id, a.intro
		HAVING COUNT(*) < 3
		   AND MAX(nl.sent_at) > NOW() - INTERVAL '7 days'
		   AND NOT EXISTS (
			   SELECT 1 FROM notification_log
			   WHERE  user_id = nl.user_id
			     AND  week_id = nl.week_id
			     AND  success = TRUE
		   )
	`)
	if err != nil {
		return nil, fmt.Errorf("get failed notifications: %w", err)
	}
	defer rows.Close()

	var result []FailedNotificationRow
	for rows.Next() {
		var r FailedNotificationRow
		if err := rows.Scan(
			&r.UserID, &r.WeekID, &r.WeekNumber,
			&r.DisplayName, &r.Email,
			&r.AnnouncementID, &r.AnnouncementIntro,
			&r.Attempts,
		); err != nil {
			return nil, fmt.Errorf("scan failed notification: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
