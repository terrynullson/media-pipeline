ALTER TABLE jobs ADD COLUMN IF NOT EXISTS payload TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS transcription_profiles (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    backend      TEXT        NOT NULL,
    model_name   TEXT        NOT NULL,
    device       TEXT        NOT NULL,
    compute_type TEXT        NOT NULL,
    language     TEXT        NOT NULL DEFAULT '',
    beam_size    INTEGER     NOT NULL,
    vad_enabled  BOOLEAN     NOT NULL DEFAULT TRUE,
    is_default   BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL
);

-- Enforce a single default profile via a partial unique index.
CREATE UNIQUE INDEX IF NOT EXISTS idx_transcription_profiles_single_default
    ON transcription_profiles(is_default)
    WHERE is_default;
