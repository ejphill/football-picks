-- +goose Up
-- +goose StatementBegin
INSERT INTO users (id, supabase_uid, display_name, email, notify_email, is_admin)
VALUES ('00000000-0000-0000-0000-000000000001', 'system', 'System', 'system@local', false, false)
ON CONFLICT DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM users WHERE id = '00000000-0000-0000-0000-000000000001';
-- +goose StatementEnd
