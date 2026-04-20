CREATE TABLE IF NOT EXISTS trigger_event_screenshots (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    media_id          BIGINT           NOT NULL REFERENCES media(id)          ON DELETE CASCADE,
    trigger_event_id  BIGINT           NOT NULL REFERENCES trigger_events(id) ON DELETE CASCADE,
    timestamp_sec     DOUBLE PRECISION NOT NULL,
    image_path        TEXT             NOT NULL,
    width             INTEGER          NOT NULL,
    height            INTEGER          NOT NULL,
    created_at        TIMESTAMPTZ      NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_trigger_event_screenshots_event
    ON trigger_event_screenshots(trigger_event_id);
CREATE INDEX IF NOT EXISTS idx_trigger_event_screenshots_media
    ON trigger_event_screenshots(media_id, trigger_event_id);
