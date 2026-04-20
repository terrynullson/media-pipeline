package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"media-pipeline/internal/domain/media"
	"media-pipeline/internal/domain/ports"
)

// Column list reused by SELECT queries — keeping it in one constant prevents
// the field order from drifting between SELECT and Scan.
const mediaColumns = `
	id, original_name, stored_name, extension, mime_type,
	size_bytes, storage_path, extracted_audio_path,
	preview_video_path, preview_video_size_bytes, preview_video_mime_type, preview_video_created_at,
	transcript_text, runtime_snapshot_json, status, created_at, updated_at,
	source_name, recording_started_at, recording_ended_at, raw_recording_label
`

type MediaRepository struct {
	db *sql.DB
}

func NewMediaRepository(db *sql.DB) *MediaRepository {
	return &MediaRepository{db: db}
}

func (r *MediaRepository) Create(ctx context.Context, m media.Media) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(
		ctx,
		`INSERT INTO media (
			original_name, stored_name, extension, mime_type,
			size_bytes, storage_path, extracted_audio_path,
			preview_video_path, preview_video_size_bytes, preview_video_mime_type, preview_video_created_at,
			transcript_text, runtime_snapshot_json, status, created_at, updated_at,
			source_name, recording_started_at, recording_ended_at, raw_recording_label
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		RETURNING id`,
		m.OriginalName,
		m.StoredName,
		m.Extension,
		m.MIMEType,
		m.SizeBytes,
		m.StoragePath,
		m.ExtractedAudioPath,
		nullIfEmpty(m.PreviewVideoPath),
		nullIfZero(m.PreviewVideoSizeBytes),
		nullIfEmpty(m.PreviewVideoMIMEType),
		nullableTime(m.PreviewVideoCreatedAtUTC),
		m.TranscriptText,
		m.RuntimeSnapshotJSON,
		m.Status,
		m.CreatedAtUTC.UTC(),
		m.UpdatedAtUTC.UTC(),
		nullIfEmpty(m.SourceName),
		nullableTime(m.RecordingStartedAtUTC),
		nullableTime(m.RecordingEndedAtUTC),
		nullIfEmpty(m.RawRecordingLabel),
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert media: %w", err)
	}

	return id, nil
}

func (r *MediaRepository) Delete(ctx context.Context, id int64) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM media WHERE id = $1", id); err != nil {
		return fmt.Errorf("delete media: %w", err)
	}
	return nil
}

func (r *MediaRepository) DeleteWithAssociations(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin media delete tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // superseded by Commit

	// transcript_segments / transcripts / trigger_events / trigger_event_screenshots /
	// transcript_windows / summaries / media_cancel_requests are all wired with
	// ON DELETE CASCADE, so the DELETE on media handles them. jobs intentionally
	// have no cascade (history may want to live on); delete them explicitly.
	if _, err := tx.ExecContext(ctx, "DELETE FROM jobs WHERE media_id = $1", id); err != nil {
		return fmt.Errorf("delete jobs by media id: %w", err)
	}

	result, err := tx.ExecContext(ctx, "DELETE FROM media WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete media by id: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete media rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("delete media by id: media %d not found", id)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit media delete tx: %w", err)
	}
	return nil
}

func (r *MediaRepository) ListRecent(ctx context.Context, limit int) ([]media.Media, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT `+mediaColumns+`
		 FROM media
		 ORDER BY created_at DESC, id DESC
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query media list: %w", err)
	}
	defer rows.Close()

	items := make([]media.Media, 0)
	for rows.Next() {
		item, err := scanMediaRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate media rows: %w", err)
	}
	return items, nil
}

func (r *MediaRepository) GetByID(ctx context.Context, id int64) (media.Media, error) {
	row := r.db.QueryRowContext(
		ctx,
		`SELECT `+mediaColumns+` FROM media WHERE id = $1`,
		id,
	)
	item, err := scanMediaRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return media.Media{}, fmt.Errorf("get media by id %d: %w", id, ports.ErrNotFound)
		}
		return media.Media{}, err
	}
	return item, nil
}

func (r *MediaRepository) MarkProcessing(ctx context.Context, id int64, nowUTC time.Time) error {
	return r.updateStatusOnly(ctx, id, media.StatusProcessing, nowUTC, "mark media processing")
}

func (r *MediaRepository) MarkUploaded(ctx context.Context, id int64, nowUTC time.Time) error {
	return r.updateStatusOnly(ctx, id, media.StatusQueued, nowUTC, "mark media uploaded")
}

func (r *MediaRepository) MarkAudioExtracted(ctx context.Context, id int64, extractedAudioPath string, nowUTC time.Time) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE media
		 SET status = $1, extracted_audio_path = $2, updated_at = $3
		 WHERE id = $4`,
		media.StatusAudioExtracted,
		extractedAudioPath,
		nowUTC.UTC(),
		id,
	)
	if err != nil {
		return fmt.Errorf("mark media audio extracted: %w", err)
	}
	return ensureRowsAffectedMedia(result, id, "mark media audio extracted")
}

func (r *MediaRepository) MarkPreviewReady(
	ctx context.Context,
	id int64,
	previewVideoPath string,
	previewVideoSizeBytes int64,
	previewVideoMIMEType string,
	previewVideoCreatedAtUTC time.Time,
	nowUTC time.Time,
) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE media
		 SET preview_video_path = $1, preview_video_size_bytes = $2, preview_video_mime_type = $3,
		     preview_video_created_at = $4, updated_at = $5
		 WHERE id = $6`,
		previewVideoPath,
		previewVideoSizeBytes,
		previewVideoMIMEType,
		previewVideoCreatedAtUTC.UTC(),
		nowUTC.UTC(),
		id,
	)
	if err != nil {
		return fmt.Errorf("mark media preview ready: %w", err)
	}
	return ensureRowsAffectedMedia(result, id, "mark media preview ready")
}

func (r *MediaRepository) MarkFailed(ctx context.Context, id int64, nowUTC time.Time) error {
	return r.updateStatusOnly(ctx, id, media.StatusFailed, nowUTC, "mark media failed")
}

func (r *MediaRepository) MarkAudioReady(ctx context.Context, id int64, nowUTC time.Time) error {
	return r.updateStatusOnly(ctx, id, media.StatusAudioExtracted, nowUTC, "mark media audio ready")
}

func (r *MediaRepository) MarkTranscribing(ctx context.Context, id int64, nowUTC time.Time) error {
	return r.updateStatusOnly(ctx, id, media.StatusTranscribing, nowUTC, "mark media transcribing")
}

func (r *MediaRepository) MarkTranscribed(ctx context.Context, id int64, transcriptText string, nowUTC time.Time) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE media
		 SET status = $1, transcript_text = $2, updated_at = $3
		 WHERE id = $4`,
		media.StatusTranscribed,
		transcriptText,
		nowUTC.UTC(),
		id,
	)
	if err != nil {
		return fmt.Errorf("mark media transcribed: %w", err)
	}
	return ensureRowsAffectedMedia(result, id, "mark media transcribed")
}

// SetRecordingEndedAt fills in the absolute end-of-broadcast timecode once the
// recording's duration is known (typically right after audio extraction or
// right after the final transcript segment is committed).
func (r *MediaRepository) SetRecordingEndedAt(ctx context.Context, id int64, endedAtUTC time.Time, nowUTC time.Time) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE media
		 SET recording_ended_at = $1, updated_at = $2
		 WHERE id = $3`,
		endedAtUTC.UTC(),
		nowUTC.UTC(),
		id,
	)
	if err != nil {
		return fmt.Errorf("set recording ended_at: %w", err)
	}
	return ensureRowsAffectedMedia(result, id, "set recording ended_at")
}

func (r *MediaRepository) updateStatusOnly(
	ctx context.Context,
	id int64,
	status media.Status,
	nowUTC time.Time,
	action string,
) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE media SET status = $1, updated_at = $2 WHERE id = $3`,
		status,
		nowUTC.UTC(),
		id,
	)
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return ensureRowsAffectedMedia(result, id, action)
}

// rowScanner unifies *sql.Row and *sql.Rows so scanMediaRow has one body.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanMediaRow(scanner rowScanner) (media.Media, error) {
	var item media.Media
	var (
		previewVideoPath      sql.NullString
		previewVideoSizeBytes sql.NullInt64
		previewVideoMIMEType  sql.NullString
		previewVideoCreatedAt sql.NullTime
		sourceName            sql.NullString
		recordingStartedAt    sql.NullTime
		recordingEndedAt      sql.NullTime
		rawRecordingLabel     sql.NullString
	)

	if err := scanner.Scan(
		&item.ID,
		&item.OriginalName,
		&item.StoredName,
		&item.Extension,
		&item.MIMEType,
		&item.SizeBytes,
		&item.StoragePath,
		&item.ExtractedAudioPath,
		&previewVideoPath,
		&previewVideoSizeBytes,
		&previewVideoMIMEType,
		&previewVideoCreatedAt,
		&item.TranscriptText,
		&item.RuntimeSnapshotJSON,
		&item.Status,
		&item.CreatedAtUTC,
		&item.UpdatedAtUTC,
		&sourceName,
		&recordingStartedAt,
		&recordingEndedAt,
		&rawRecordingLabel,
	); err != nil {
		return media.Media{}, err
	}

	item.CreatedAtUTC = item.CreatedAtUTC.UTC()
	item.UpdatedAtUTC = item.UpdatedAtUTC.UTC()

	if previewVideoPath.Valid {
		item.PreviewVideoPath = previewVideoPath.String
	}
	if previewVideoSizeBytes.Valid {
		item.PreviewVideoSizeBytes = previewVideoSizeBytes.Int64
	}
	if previewVideoMIMEType.Valid {
		item.PreviewVideoMIMEType = previewVideoMIMEType.String
	}
	if previewVideoCreatedAt.Valid {
		t := previewVideoCreatedAt.Time.UTC()
		item.PreviewVideoCreatedAtUTC = &t
	}
	if sourceName.Valid {
		item.SourceName = sourceName.String
	}
	if recordingStartedAt.Valid {
		t := recordingStartedAt.Time.UTC()
		item.RecordingStartedAtUTC = &t
	}
	if recordingEndedAt.Valid {
		t := recordingEndedAt.Time.UTC()
		item.RecordingEndedAtUTC = &t
	}
	if rawRecordingLabel.Valid {
		item.RawRecordingLabel = rawRecordingLabel.String
	}
	return item, nil
}

func ensureRowsAffectedMedia(result sql.Result, id int64, action string) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s rows affected: %w", action, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("%s: media %d not found", action, id)
	}
	return nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullIfZero(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}

// nullableTime returns a typed nil for nil/zero times so the pgx driver writes
// SQL NULL (not "0001-01-01 00:00:00").
func nullableTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC()
}
