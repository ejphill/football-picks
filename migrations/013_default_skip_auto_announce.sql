-- +goose Up
-- +goose StatementBegin
-- Auto-send relies on Resend, which currently has no verified sending
-- domain — until that's sorted out, default every week (existing and
-- future) to skip the automatic Saturday send. Jack is manually copying
-- the drafted announcement and sending it himself in the meantime.
ALTER TABLE weeks ALTER COLUMN skip_auto_announce SET DEFAULT TRUE;
UPDATE weeks SET skip_auto_announce = TRUE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE weeks ALTER COLUMN skip_auto_announce SET DEFAULT FALSE;
-- +goose StatementEnd
