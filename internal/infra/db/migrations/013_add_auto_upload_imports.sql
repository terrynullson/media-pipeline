CREATE TABLE IF NOT EXISTS auto_upload_imports (
    fingerprint TEXT PRIMARY KEY,
    source_path TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    modified_at TEXT NOT NULL,
    status TEXT NOT NULL,
    media_id INTEGER,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_auto_upload_imports_status ON auto_upload_imports(status);
