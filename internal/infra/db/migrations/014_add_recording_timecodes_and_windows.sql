-- Broadcast / airtime analytics block.
--
-- Goal: every airtime recording carries an absolute recording_started_at /
-- recording_ended_at; every transcript segment carries the absolute wall-clock
-- time of when it was uttered; pre-aggregated transcript_windows let the UI
-- and exporters answer "what was said between HH:MM and HH:MM" without
-- recomputing from raw segments at query time.

ALTER TABLE media
    ADD COLUMN IF NOT EXISTS source_name           TEXT,
    ADD COLUMN IF NOT EXISTS recording_started_at  TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS recording_ended_at    TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS raw_recording_label   TEXT;

CREATE INDEX IF NOT EXISTS idx_media_recording_started_at ON media(recording_started_at);
CREATE INDEX IF NOT EXISTS idx_media_recording_ended_at   ON media(recording_ended_at);
CREATE INDEX IF NOT EXISTS idx_media_source_name          ON media(source_name);

ALTER TABLE transcript_segments
    ADD COLUMN IF NOT EXISTS segment_started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS segment_ended_at   TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_transcript_segments_segment_started_at
    ON transcript_segments(segment_started_at);

CREATE TABLE IF NOT EXISTS transcript_windows (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    media_id          BIGINT      NOT NULL REFERENCES media(id)       ON DELETE CASCADE,
    transcript_id     BIGINT      NOT NULL REFERENCES transcripts(id) ON DELETE CASCADE,
    window_size_sec   INTEGER     NOT NULL CHECK (window_size_sec > 0),
    window_started_at TIMESTAMPTZ NOT NULL,
    window_ended_at   TIMESTAMPTZ NOT NULL,
    text              TEXT        NOT NULL,
    segment_count     INTEGER     NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_transcript_windows_unique
    ON transcript_windows(transcript_id, window_size_sec, window_started_at);
CREATE INDEX IF NOT EXISTS idx_transcript_windows_started_at
    ON transcript_windows(window_started_at, window_size_sec);
CREATE INDEX IF NOT EXISTS idx_transcript_windows_media
    ON transcript_windows(media_id, window_size_sec, window_started_at);
