-- +goose Up
-- +goose StatementBegin

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    supabase_uid  TEXT UNIQUE NOT NULL,
    display_name  TEXT NOT NULL,
    email         TEXT UNIQUE NOT NULL,
    notify_email  BOOLEAN NOT NULL DEFAULT TRUE,
    is_admin      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE seasons (
    id         SERIAL PRIMARY KEY,
    year       INT UNIQUE NOT NULL,
    is_active  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE weeks (
    id            SERIAL PRIMARY KEY,
    season_id     INT NOT NULL REFERENCES seasons(id),
    week_number   INT NOT NULL,
    picks_lock_at TIMESTAMPTZ NOT NULL,
    UNIQUE(season_id, week_number)
);

CREATE TABLE games (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    week_id         INT NOT NULL REFERENCES weeks(id),
    espn_game_id    TEXT UNIQUE NOT NULL,
    home_team       TEXT NOT NULL,
    away_team       TEXT NOT NULL,
    home_team_name  TEXT NOT NULL,
    away_team_name  TEXT NOT NULL,
    spread          NUMERIC(4,1),
    kickoff_at      TIMESTAMPTZ NOT NULL,
    home_score      INT,
    away_score      INT,
    winner          TEXT,
    status          TEXT NOT NULL DEFAULT 'scheduled',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE picks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id),
    game_id     UUID NOT NULL REFERENCES games(id),
    picked_team TEXT NOT NULL,
    is_correct  BOOLEAN,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, game_id)
);

CREATE TABLE announcements (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id    UUID NOT NULL REFERENCES users(id),
    week_id      INT NOT NULL REFERENCES weeks(id),
    intro        TEXT NOT NULL,
    body_json    JSONB,
    published_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE notification_log (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id   UUID NOT NULL REFERENCES users(id),
    week_id   INT NOT NULL REFERENCES weeks(id),
    sent_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    success   BOOLEAN NOT NULL,
    error_msg TEXT
);

-- Indexes
CREATE INDEX idx_picks_user_id ON picks(user_id);
CREATE INDEX idx_picks_game_id ON picks(game_id);
CREATE INDEX idx_games_week_id ON games(week_id);
CREATE INDEX idx_picks_is_correct ON picks(is_correct);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS notification_log;
DROP TABLE IF EXISTS announcements;
DROP TABLE IF EXISTS picks;
DROP TABLE IF EXISTS games;
DROP TABLE IF EXISTS weeks;
DROP TABLE IF EXISTS seasons;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
