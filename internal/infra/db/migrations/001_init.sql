CREATE TABLE IF NOT EXISTS media (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    original_name   TEXT        NOT NULL,
    stored_name     TEXT        NOT NULL,
    extension       TEXT        NOT NULL,
    mime_type       TEXT        NOT NULL,
    size_bytes      BIGINT      NOT NULL,
    storage_path    TEXT        NOT NULL,
    status          TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS jobs (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    media_id      BIGINT      NOT NULL REFERENCES media(id),
    type          TEXT        NOT NULL,
    status        TEXT        NOT NULL,
    attempts      INTEGER     NOT NULL DEFAULT 0,
    error_message TEXT,
    created_at    TIMESTAMPTZ NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_media_created_at ON media(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_media_id    ON jobs(media_id);
CREATE INDEX IF NOT EXISTS idx_jobs_status_type ON jobs(status, type);
