-- +goose Up
-- +goose StatementBegin
-- Caches the announcement's "season records" text, computed once the first
-- time anything asks for a given week's draft, then read directly
-- thereafter — avoids recomputing the floor-scoring standings query (CTEs,
-- cross joins) on every Home page visit / compose page load for the same
-- week. Deliberately frozen, not a TTL cache: this is supposed to represent
-- "standings as of the start of this week," which shouldn't change as the
-- week progresses.
ALTER TABLE weeks ADD COLUMN frozen_records TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE weeks DROP COLUMN frozen_records;
-- +goose StatementEnd
