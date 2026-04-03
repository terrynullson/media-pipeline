ALTER TABLE jobs ADD COLUMN started_at TEXT;
ALTER TABLE jobs ADD COLUMN finished_at TEXT;
ALTER TABLE jobs ADD COLUMN duration_ms INTEGER;
ALTER TABLE jobs ADD COLUMN progress_percent REAL;
ALTER TABLE jobs ADD COLUMN progress_label TEXT NOT NULL DEFAULT '';
ALTER TABLE jobs ADD COLUMN progress_is_estimate INTEGER NOT NULL DEFAULT 0;
ALTER TABLE jobs ADD COLUMN progress_updated_at TEXT;

ALTER TABLE media ADD COLUMN runtime_snapshot_json TEXT NOT NULL DEFAULT '';
