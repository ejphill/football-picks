-- +goose Up
-- +goose StatementBegin

-- migration 009 set picks_lock_at to each week's *start* (Sunday 1pm ET),
-- which made GetActiveWeek flip to the next week before that week's Sunday
-- night / Monday night games had even been played. picks_lock_at is only
-- ever used to pick the "active" week for display — it doesn't lock
-- anything (that's games.kickoff_at, per-game) — so it should mark the end
-- of a week, not the start. Shifting to Tuesday noon ET, after MNF wraps.
-- +2 days -1 hour: Sunday 1pm ET -> Tuesday 12pm ET.
UPDATE weeks w
SET picks_lock_at = w.picks_lock_at + INTERVAL '1 day 23 hours'
FROM seasons s
WHERE w.season_id = s.id AND s.year = 2026;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE weeks w
SET picks_lock_at = w.picks_lock_at - INTERVAL '1 day 23 hours'
FROM seasons s
WHERE w.season_id = s.id AND s.year = 2026;
-- +goose StatementEnd
