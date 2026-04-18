CREATE TABLE IF NOT EXISTS runtime_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    auto_upload_min_age_sec INTEGER NOT NULL DEFAULT 60,
    preview_timeout_sec INTEGER NOT NULL DEFAULT 600,
    max_upload_size_mb INTEGER NOT NULL DEFAULT 1024,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
