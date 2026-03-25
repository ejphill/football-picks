-- +goose Up

-- Prevents duplicate successful notifications per user per week.
-- LogNotification uses ON CONFLICT DO NOTHING so retries don't error.
CREATE UNIQUE INDEX idx_notification_log_user_week_success
    ON notification_log(user_id, week_id)
    WHERE success = TRUE;

-- Prevents two scheduler instances auto-posting the same week's announcement.
-- CreateAutoAnnouncement uses ON CONFLICT DO NOTHING and treats a nil RETURNING
-- result as "another instance won".
CREATE UNIQUE INDEX idx_announcements_auto_week
    ON announcements(week_id)
    WHERE author_id = '00000000-0000-0000-0000-000000000001';

-- +goose Down
DROP INDEX IF EXISTS idx_notification_log_user_week_success;
DROP INDEX IF EXISTS idx_announcements_auto_week;
