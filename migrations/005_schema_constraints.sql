-- +goose Up

-- Enum-like CHECK constraints that were missing from the initial schema.
ALTER TABLE games
    ADD CONSTRAINT games_winner_check  CHECK (winner   IN ('home', 'away', 'tie')),
    ADD CONSTRAINT games_status_check  CHECK (status   IN ('scheduled', 'in_progress', 'final'));

ALTER TABLE picks
    ADD CONSTRAINT picks_picked_team_check CHECK (picked_team IN ('home', 'away'));

-- Replace low-selectivity boolean index with a partial index that matches
-- the actual query pattern in ScorePicks.
DROP INDEX IF EXISTS idx_picks_is_correct;
CREATE INDEX idx_picks_unscored ON picks(game_id) WHERE is_correct IS NULL;

-- body_json is always scanned as plain text; use TEXT to match the Go model.
ALTER TABLE announcements ALTER COLUMN body_json TYPE TEXT USING body_json::TEXT;

-- +goose Down
ALTER TABLE games
    DROP CONSTRAINT IF EXISTS games_winner_check,
    DROP CONSTRAINT IF EXISTS games_status_check;
ALTER TABLE picks DROP CONSTRAINT IF EXISTS picks_picked_team_check;
DROP INDEX IF EXISTS idx_picks_unscored;
CREATE INDEX idx_picks_is_correct ON picks(is_correct);
ALTER TABLE announcements ALTER COLUMN body_json TYPE JSONB USING body_json::JSONB;
