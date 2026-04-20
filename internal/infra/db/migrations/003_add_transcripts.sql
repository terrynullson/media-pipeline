ALTER TABLE media ADD COLUMN IF NOT EXISTS transcript_text TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS transcripts (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    media_id    BIGINT      NOT NULL UNIQUE REFERENCES media(id) ON DELETE CASCADE,
    language    TEXT        NOT NULL DEFAULT '',
    full_text   TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS transcript_segments (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    transcript_id  BIGINT           NOT NULL REFERENCES transcripts(id) ON DELETE CASCADE,
    segment_index  INTEGER          NOT NULL,
    start_sec      DOUBLE PRECISION NOT NULL,
    end_sec        DOUBLE PRECISION NOT NULL,
    text           TEXT             NOT NULL,
    confidence     DOUBLE PRECISION,
    created_at     TIMESTAMPTZ      NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_transcripts_media_id ON transcripts(media_id);
CREATE INDEX IF NOT EXISTS idx_transcript_segments_transcript_id
    ON transcript_segments(transcript_id, segment_index);
