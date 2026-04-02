CREATE TABLE IF NOT EXISTS users (
    id           SERIAL PRIMARY KEY,
    email        VARCHAR(255) UNIQUE  NOT NULL,
    password_hash TEXT,
    google_id    VARCHAR(255) UNIQUE,
    name         VARCHAR(255)         NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ          NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ          NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users (deleted_at);