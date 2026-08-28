-- +goose Up
-- +goose StatementBegin

-- Deactivate 2025 first: idx_seasons_one_active (migration 008) allows only
-- one active season at a time.
UPDATE seasons SET is_active = false WHERE year = 2025;

INSERT INTO seasons (year, is_active) VALUES (2026, true)
ON CONFLICT (year) DO UPDATE SET is_active = EXCLUDED.is_active;

-- picks_lock_at here is a nominal week boundary used for "current week"
-- sorting (queries.GetActiveWeek); the real per-game lock is games.kickoff_at,
-- populated by the ESPN sync. Dates below are estimated 1pm ET Sundays
-- (Thursday-opener cadence, week after Labor Day) — adjust once the actual
-- 2026 schedule is released.
INSERT INTO weeks (season_id, week_number, picks_lock_at)
SELECT s.id, w.week_number, w.picks_lock_at
FROM seasons s,
(VALUES
    (1,  TIMESTAMPTZ '2026-09-13 17:00:00+00'),
    (2,  TIMESTAMPTZ '2026-09-20 17:00:00+00'),
    (3,  TIMESTAMPTZ '2026-09-27 17:00:00+00'),
    (4,  TIMESTAMPTZ '2026-10-04 17:00:00+00'),
    (5,  TIMESTAMPTZ '2026-10-11 17:00:00+00'),
    (6,  TIMESTAMPTZ '2026-10-18 17:00:00+00'),
    (7,  TIMESTAMPTZ '2026-10-25 17:00:00+00'),
    (8,  TIMESTAMPTZ '2026-11-01 18:00:00+00'),
    (9,  TIMESTAMPTZ '2026-11-08 18:00:00+00'),
    (10, TIMESTAMPTZ '2026-11-15 18:00:00+00'),
    (11, TIMESTAMPTZ '2026-11-22 18:00:00+00'),
    (12, TIMESTAMPTZ '2026-11-29 18:00:00+00'),
    (13, TIMESTAMPTZ '2026-12-06 18:00:00+00'),
    (14, TIMESTAMPTZ '2026-12-13 18:00:00+00'),
    (15, TIMESTAMPTZ '2026-12-20 18:00:00+00'),
    (16, TIMESTAMPTZ '2026-12-27 18:00:00+00'),
    (17, TIMESTAMPTZ '2027-01-03 18:00:00+00'),
    (18, TIMESTAMPTZ '2027-01-10 18:00:00+00')
) AS w(week_number, picks_lock_at)
WHERE s.year = 2026
ON CONFLICT (season_id, week_number) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM weeks WHERE season_id = (SELECT id FROM seasons WHERE year = 2026);
DELETE FROM seasons WHERE year = 2026;
UPDATE seasons SET is_active = true WHERE year = 2025;
-- +goose StatementEnd
