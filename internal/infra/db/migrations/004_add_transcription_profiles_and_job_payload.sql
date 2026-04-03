ALTER TABLE jobs ADD COLUMN payload TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS transcription_profiles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    backend TEXT NOT NULL,
    model_name TEXT NOT NULL,
    device TEXT NOT NULL,
    compute_type TEXT NOT NULL,
    language TEXT NOT NULL DEFAULT '',
    beam_size INTEGER NOT NULL,
    vad_enabled INTEGER NOT NULL DEFAULT 1,
    is_default INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_transcription_profiles_single_default
    ON transcription_profiles(is_default)
    WHERE is_default = 1;
