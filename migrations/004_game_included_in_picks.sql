-- +goose Up
ALTER TABLE games ADD COLUMN included_in_picks BOOLEAN NOT NULL DEFAULT TRUE;

-- Back-fill: exclude Thursday and Friday games already in the table.
UPDATE games
SET included_in_picks = FALSE
WHERE EXTRACT(DOW FROM kickoff_at AT TIME ZONE 'America/New_York') IN (5, 4);
-- DOW: 0=Sunday, 1=Monday, ..., 4=Thursday, 5=Friday

-- +goose Down
ALTER TABLE games DROP COLUMN included_in_picks;
