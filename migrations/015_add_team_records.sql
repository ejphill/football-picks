-- +goose Up
ALTER TABLE games ADD COLUMN home_team_record TEXT;
ALTER TABLE games ADD COLUMN away_team_record TEXT;

-- +goose Down
ALTER TABLE games DROP COLUMN home_team_record;
ALTER TABLE games DROP COLUMN away_team_record;
