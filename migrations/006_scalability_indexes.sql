-- +goose Up

-- Week lookups: GetWeekByNumberAndSeason joins weeks→seasons on season_id.
-- Without this, every week lookup (active week, sync, pick submission) does a
-- full scan of weeks. The UNIQUE(season_id, week_number) constraint already
-- covers the pair for exact lookups, but this index supports the FK join path.
CREATE INDEX idx_weeks_season_id ON weeks(season_id);

-- ScorePicks filters games WHERE status = 'final'. At 60s poll cadence during
-- game windows this runs constantly. Partial index excludes the majority of
-- rows (scheduled/in_progress) that are never matched by ScorePicks.
CREATE INDEX idx_games_status_final ON games(status) WHERE status = 'final';

-- Poller kickoff window check: syncIfNeeded scans games ordered/filtered by
-- kickoff_at to find the next game within the hour. Supports both the range
-- filter and ORDER BY kickoff_at in GetGamesByWeek.
CREATE INDEX idx_games_kickoff_at ON games(kickoff_at);

-- Announcement lookups by week (GetAnnouncementsByWeek, scheduler check
-- before auto-send, draft building). Week_id appears in both WHERE clauses.
CREATE INDEX idx_announcements_week_id ON announcements(week_id);

-- GetUsersForNotification: SELECT * FROM users WHERE notify_email = TRUE.
-- Partial index covers exactly the rows returned; skips the FALSE majority
-- as the user base grows and more users opt out.
CREATE INDEX idx_users_notify_email ON users(notify_email) WHERE notify_email = TRUE;

-- Notification log lookups by week (duplicate-send prevention, audit queries).
CREATE INDEX idx_notification_log_week_id ON notification_log(week_id);

-- +goose Down
DROP INDEX IF EXISTS idx_weeks_season_id;
DROP INDEX IF EXISTS idx_games_status_final;
DROP INDEX IF EXISTS idx_games_kickoff_at;
DROP INDEX IF EXISTS idx_announcements_week_id;
DROP INDEX IF EXISTS idx_users_notify_email;
DROP INDEX IF EXISTS idx_notification_log_week_id;
