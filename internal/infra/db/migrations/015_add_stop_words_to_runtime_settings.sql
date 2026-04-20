-- Adds configurable stop-words for the analytics word-frequency report.
-- Stored as a newline-separated TEXT blob (simple, readable, editable in UI).

ALTER TABLE runtime_settings
    ADD COLUMN IF NOT EXISTS stop_words TEXT NOT NULL DEFAULT '';
