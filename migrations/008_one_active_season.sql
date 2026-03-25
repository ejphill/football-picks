-- +goose Up

-- Enforces at most one active season at the DB level.
-- Allows unlimited FALSE rows; only one TRUE is permitted.
CREATE UNIQUE INDEX idx_seasons_one_active ON seasons(is_active) WHERE is_active = TRUE;

-- +goose Down
DROP INDEX IF EXISTS idx_seasons_one_active;
