ALTER TABLE media ADD COLUMN IF NOT EXISTS preview_video_path       TEXT;
ALTER TABLE media ADD COLUMN IF NOT EXISTS preview_video_size_bytes BIGINT;
ALTER TABLE media ADD COLUMN IF NOT EXISTS preview_video_mime_type  TEXT;
ALTER TABLE media ADD COLUMN IF NOT EXISTS preview_video_created_at TIMESTAMPTZ;
