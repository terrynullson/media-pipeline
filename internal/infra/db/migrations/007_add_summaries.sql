CREATE TABLE IF NOT EXISTS summaries (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    media_id        BIGINT      NOT NULL UNIQUE REFERENCES media(id) ON DELETE CASCADE,
    summary_text    TEXT        NOT NULL,
    highlights_json TEXT        NOT NULL DEFAULT '[]',
    provider        TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_summaries_media_id ON summaries(media_id);
