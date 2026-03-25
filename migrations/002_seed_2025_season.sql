-- +goose Up
-- +goose StatementBegin

INSERT INTO seasons (year, is_active) VALUES (2025, true)
ON CONFLICT (year) DO UPDATE SET is_active = EXCLUDED.is_active;

INSERT INTO weeks (season_id, week_number, picks_lock_at)
SELECT s.id, w.week_number, w.picks_lock_at
FROM seasons s,
(VALUES
    (1,  TIMESTAMPTZ '2025-09-07 17:00:00+00'),
    (2,  TIMESTAMPTZ '2025-09-14 17:00:00+00'),
    (3,  TIMESTAMPTZ '2025-09-21 17:00:00+00'),
    (4,  TIMESTAMPTZ '2025-09-28 17:00:00+00'),
    (5,  TIMESTAMPTZ '2025-10-05 17:00:00+00'),
    (6,  TIMESTAMPTZ '2025-10-12 17:00:00+00'),
    (7,  TIMESTAMPTZ '2025-10-19 17:00:00+00'),
    (8,  TIMESTAMPTZ '2025-10-26 17:00:00+00'),
    (9,  TIMESTAMPTZ '2025-11-02 18:00:00+00'),
    (10, TIMESTAMPTZ '2025-11-09 18:00:00+00'),
    (11, TIMESTAMPTZ '2025-11-16 18:00:00+00'),
    (12, TIMESTAMPTZ '2025-11-23 18:00:00+00'),
    (13, TIMESTAMPTZ '2025-11-30 18:00:00+00'),
    (14, TIMESTAMPTZ '2025-12-07 18:00:00+00'),
    (15, TIMESTAMPTZ '2025-12-14 18:00:00+00'),
    (16, TIMESTAMPTZ '2025-12-21 18:00:00+00'),
    (17, TIMESTAMPTZ '2025-12-28 18:00:00+00'),
    (18, TIMESTAMPTZ '2026-01-04 18:00:00+00')
) AS w(week_number, picks_lock_at)
WHERE s.year = 2025
ON CONFLICT (season_id, week_number) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM weeks WHERE season_id = (SELECT id FROM seasons WHERE year = 2025);
DELETE FROM seasons WHERE year = 2025;
-- +goose StatementEnd
