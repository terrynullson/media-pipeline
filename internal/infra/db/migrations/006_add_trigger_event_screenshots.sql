CREATE TABLE IF NOT EXISTS trigger_event_screenshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    trigger_event_id INTEGER NOT NULL,
    timestamp_sec REAL NOT NULL,
    image_path TEXT NOT NULL,
    width INTEGER NOT NULL,
    height INTEGER NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    FOREIGN KEY (trigger_event_id) REFERENCES trigger_events(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_trigger_event_screenshots_event
    ON trigger_event_screenshots(trigger_event_id);
CREATE INDEX IF NOT EXISTS idx_trigger_event_screenshots_media
    ON trigger_event_screenshots(media_id, trigger_event_id);
