CREATE TABLE IF NOT EXISTS trigger_rules (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        TEXT        NOT NULL UNIQUE,
    category    TEXT        NOT NULL,
    pattern     TEXT        NOT NULL,
    match_mode  TEXT        NOT NULL,
    enabled     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_trigger_rules_enabled ON trigger_rules(enabled);

CREATE TABLE IF NOT EXISTS trigger_events (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    media_id       BIGINT           NOT NULL REFERENCES media(id)        ON DELETE CASCADE,
    transcript_id  BIGINT                    REFERENCES transcripts(id)  ON DELETE CASCADE,
    rule_id        BIGINT           NOT NULL REFERENCES trigger_rules(id) ON DELETE CASCADE,
    category       TEXT             NOT NULL,
    matched_text   TEXT             NOT NULL,
    segment_index  INTEGER          NOT NULL,
    start_sec      DOUBLE PRECISION NOT NULL,
    end_sec        DOUBLE PRECISION NOT NULL,
    segment_text   TEXT             NOT NULL,
    context_text   TEXT             NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ      NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_trigger_events_media_rule_segment
    ON trigger_events(media_id, rule_id, segment_index);
CREATE INDEX IF NOT EXISTS idx_trigger_events_media_time
    ON trigger_events(media_id, start_sec, id);
CREATE INDEX IF NOT EXISTS idx_trigger_events_transcript_segment
    ON trigger_events(transcript_id, segment_index);

INSERT INTO trigger_rules (name, category, pattern, match_mode, enabled, created_at, updated_at)
VALUES
    ('Escalation Request',  'support',   'speak to a manager',     'contains', TRUE, NOW(), NOW()),
    ('Billing Complaint',   'billing',   'refund',                 'contains', TRUE, NOW(), NOW()),
    ('Cancellation Intent', 'retention', 'cancel my subscription', 'contains', TRUE, NOW(), NOW())
ON CONFLICT (name) DO NOTHING;
