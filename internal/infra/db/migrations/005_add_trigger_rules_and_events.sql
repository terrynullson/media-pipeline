CREATE TABLE IF NOT EXISTS trigger_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    category TEXT NOT NULL,
    pattern TEXT NOT NULL,
    match_mode TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_trigger_rules_enabled ON trigger_rules(enabled);

CREATE TABLE IF NOT EXISTS trigger_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    transcript_id INTEGER,
    rule_id INTEGER NOT NULL,
    category TEXT NOT NULL,
    matched_text TEXT NOT NULL,
    segment_index INTEGER NOT NULL,
    start_sec REAL NOT NULL,
    end_sec REAL NOT NULL,
    segment_text TEXT NOT NULL,
    context_text TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    FOREIGN KEY (transcript_id) REFERENCES transcripts(id) ON DELETE CASCADE,
    FOREIGN KEY (rule_id) REFERENCES trigger_rules(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_trigger_events_media_rule_segment
    ON trigger_events(media_id, rule_id, segment_index);
CREATE INDEX IF NOT EXISTS idx_trigger_events_media_time
    ON trigger_events(media_id, start_sec, id);
CREATE INDEX IF NOT EXISTS idx_trigger_events_transcript_segment
    ON trigger_events(transcript_id, segment_index);

INSERT OR IGNORE INTO trigger_rules (name, category, pattern, match_mode, enabled, created_at, updated_at)
VALUES
    ('Escalation Request', 'support', 'speak to a manager', 'contains', 1, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    ('Billing Complaint', 'billing', 'refund', 'contains', 1, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    ('Cancellation Intent', 'retention', 'cancel my subscription', 'contains', 1, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));
