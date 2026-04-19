-- Singleton table: enforced via PK + CHECK (id = 1).
CREATE TABLE IF NOT EXISTS runtime_settings (
    id                       SMALLINT    NOT NULL PRIMARY KEY CHECK (id = 1),
    auto_upload_min_age_sec  BIGINT      NOT NULL DEFAULT 60,
    preview_timeout_sec      BIGINT      NOT NULL DEFAULT 600,
    max_upload_size_mb       BIGINT      NOT NULL DEFAULT 1024,
    created_at               TIMESTAMPTZ NOT NULL,
    updated_at               TIMESTAMPTZ NOT NULL
);
