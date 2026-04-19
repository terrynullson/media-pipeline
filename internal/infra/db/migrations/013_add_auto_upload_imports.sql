CREATE TABLE IF NOT EXISTS auto_upload_imports (
    fingerprint TEXT        PRIMARY KEY,
    source_path TEXT        NOT NULL,
    size_bytes  BIGINT      NOT NULL,
    modified_at TIMESTAMPTZ NOT NULL,
    status      TEXT        NOT NULL,
    media_id    BIGINT      REFERENCES media(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_auto_upload_imports_status ON auto_upload_imports(status);
