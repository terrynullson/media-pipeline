CREATE TABLE IF NOT EXISTS media_cancel_requests (
    media_id     BIGINT      PRIMARY KEY REFERENCES media(id) ON DELETE CASCADE,
    requested_at TIMESTAMPTZ NOT NULL
);
