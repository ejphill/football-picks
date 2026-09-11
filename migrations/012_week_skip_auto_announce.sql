-- +goose Up
-- +goose StatementBegin
ALTER TABLE weeks ADD COLUMN skip_auto_announce BOOLEAN NOT NULL DEFAULT FALSE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE weeks DROP COLUMN skip_auto_announce;
-- +goose StatementEnd
