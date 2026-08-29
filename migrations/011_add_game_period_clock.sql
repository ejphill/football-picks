-- +goose Up
-- +goose StatementBegin
ALTER TABLE games ADD COLUMN period INT;
ALTER TABLE games ADD COLUMN display_clock TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE games DROP COLUMN period;
ALTER TABLE games DROP COLUMN display_clock;
-- +goose StatementEnd
