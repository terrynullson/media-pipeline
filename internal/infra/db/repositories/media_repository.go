package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"media-pipeline/internal/domain/media"
)

type MediaRepository struct {
	db *sql.DB
}

func NewMediaRepository(db *sql.DB) *MediaRepository {
	return &MediaRepository{db: db}
}

func (r *MediaRepository) Create(ctx context.Context, m media.Media) (int64, error) {
	result, err := r.db.ExecContext(
		ctx,
		`INSERT INTO media (
			original_name, stored_name, extension, mime_type,
			size_bytes, storage_path, extracted_audio_path, preview_video_path, preview_video_size_bytes, preview_video_mime_type, preview_video_created_at, transcript_text, runtime_snapshot_json, status, created_at, updated_at
		 ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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
		formatOptionalTime(m.PreviewVideoCreatedAtUTC),
		m.TranscriptText,
		m.RuntimeSnapshotJSON,
		m.Status,
		m.CreatedAtUTC.Format(time.RFC3339),
		m.UpdatedAtUTC.Format(time.RFC3339),
	)
	if err != nil {
		return 0, fmt.Errorf("insert media: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("media last insert id: %w", err)
	}

	return id, nil
}

func (r *MediaRepository) Delete(ctx context.Context, id int64) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM media WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete media: %w", err)
	}

	return nil
}

func (r *MediaRepository) DeleteWithAssociations(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin media delete tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM transcript_segments
		 WHERE transcript_id IN (
		 	SELECT id
		 	FROM transcripts
		 	WHERE media_id = ?
		 )`,
		id,
	); err != nil {
		return fmt.Errorf("delete transcript segments by media id: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM transcripts WHERE media_id = ?", id); err != nil {
		return fmt.Errorf("delete transcripts by media id: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM jobs WHERE media_id = ?", id); err != nil {
		return fmt.Errorf("delete jobs by media id: %w", err)
	}

	result, err := tx.ExecContext(ctx, "DELETE FROM media WHERE id = ?", id)
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
		`SELECT id, original_name, stored_name, extension, mime_type,
			size_bytes, storage_path, extracted_audio_path, preview_video_path, preview_video_size_bytes, preview_video_mime_type, preview_video_created_at, transcript_text, runtime_snapshot_json, status, created_at, updated_at
		 FROM media
		 ORDER BY datetime(created_at) DESC
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query media list: %w", err)
	}
	defer rows.Close()

	items := make([]media.Media, 0)
	for rows.Next() {
		var item media.Media
		var createdAt, updatedAt string
		var previewVideoPath sql.NullString
		var previewVideoSizeBytes sql.NullInt64
		var previewVideoMIMEType sql.NullString
		var previewVideoCreatedAt sql.NullString
		if scanErr := rows.Scan(
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
			&createdAt,
			&updatedAt,
		); scanErr != nil {
			return nil, fmt.Errorf("scan media row: %w", scanErr)
		}

		item.CreatedAtUTC, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse media created_at: %w", err)
		}
		item.UpdatedAtUTC, err = time.Parse(time.RFC3339, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse media updated_at: %w", err)
		}
		applyPreviewFields(&item, previewVideoPath, previewVideoSizeBytes, previewVideoMIMEType, previewVideoCreatedAt)
		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate media rows: %w", err)
	}

	return items, nil
}

func (r *MediaRepository) GetByID(ctx context.Context, id int64) (media.Media, error) {
	row := r.db.QueryRowContext(
		ctx,
		`SELECT id, original_name, stored_name, extension, mime_type,
			size_bytes, storage_path, extracted_audio_path, preview_video_path, preview_video_size_bytes, preview_video_mime_type, preview_video_created_at, transcript_text, runtime_snapshot_json, status, created_at, updated_at
		 FROM media
		 WHERE id = ?`,
		id,
	)

	var item media.Media
	var createdAt string
	var updatedAt string
	var previewVideoPath sql.NullString
	var previewVideoSizeBytes sql.NullInt64
	var previewVideoMIMEType sql.NullString
	var previewVideoCreatedAt sql.NullString
	if err := row.Scan(
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
		&createdAt,
		&updatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return media.Media{}, fmt.Errorf("get media by id %d: %w", id, err)
		}
		return media.Media{}, fmt.Errorf("scan media by id %d: %w", id, err)
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return media.Media{}, fmt.Errorf("parse media created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return media.Media{}, fmt.Errorf("parse media updated_at: %w", err)
	}
	item.CreatedAtUTC = parsedCreatedAt
	item.UpdatedAtUTC = parsedUpdatedAt
	applyPreviewFields(&item, previewVideoPath, previewVideoSizeBytes, previewVideoMIMEType, previewVideoCreatedAt)

	return item, nil
}

func (r *MediaRepository) MarkProcessing(ctx context.Context, id int64, nowUTC time.Time) error {
	return r.updateStatusOnly(ctx, id, media.StatusProcessing, nowUTC, "mark media processing")
}

func (r *MediaRepository) MarkUploaded(ctx context.Context, id int64, nowUTC time.Time) error {
	return r.updateStatusOnly(ctx, id, media.StatusUploaded, nowUTC, "mark media uploaded")
}

func (r *MediaRepository) MarkAudioExtracted(ctx context.Context, id int64, extractedAudioPath string, nowUTC time.Time) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE media
		 SET status = ?, extracted_audio_path = ?, updated_at = ?
		 WHERE id = ?`,
		media.StatusAudioExtracted,
		extractedAudioPath,
		nowUTC.Format(time.RFC3339),
		id,
	)
	if err != nil {
		return fmt.Errorf("mark media audio extracted: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("media audio extracted rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("mark media audio extracted: media %d not found", id)
	}

	return nil
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
		 SET preview_video_path = ?, preview_video_size_bytes = ?, preview_video_mime_type = ?, preview_video_created_at = ?, updated_at = ?
		 WHERE id = ?`,
		previewVideoPath,
		previewVideoSizeBytes,
		previewVideoMIMEType,
		previewVideoCreatedAtUTC.Format(time.RFC3339),
		nowUTC.Format(time.RFC3339),
		id,
	)
	if err != nil {
		return fmt.Errorf("mark media preview ready: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("media preview ready rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("mark media preview ready: media %d not found", id)
	}

	return nil
}

func (r *MediaRepository) MarkFailed(ctx context.Context, id int64, nowUTC time.Time) error {
	return r.updateStatusOnly(ctx, id, media.StatusFailed, nowUTC, "mark media failed")
}

func (r *MediaRepository) MarkAudioReady(ctx context.Context, id int64, nowUTC time.Time) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE media
		 SET status = ?, updated_at = ?
		 WHERE id = ?`,
		media.StatusAudioExtracted,
		nowUTC.Format(time.RFC3339),
		id,
	)
	if err != nil {
		return fmt.Errorf("mark media audio ready: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("media audio ready rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("mark media audio ready: media %d not found", id)
	}

	return nil
}

func (r *MediaRepository) MarkTranscribing(ctx context.Context, id int64, nowUTC time.Time) error {
	return r.updateStatusOnly(ctx, id, media.StatusTranscribing, nowUTC, "mark media transcribing")
}

func (r *MediaRepository) MarkTranscribed(ctx context.Context, id int64, transcriptText string, nowUTC time.Time) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE media
		 SET status = ?, transcript_text = ?, updated_at = ?
		 WHERE id = ?`,
		media.StatusTranscribed,
		transcriptText,
		nowUTC.Format(time.RFC3339),
		id,
	)
	if err != nil {
		return fmt.Errorf("mark media transcribed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("media transcribed rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("mark media transcribed: media %d not found", id)
	}

	return nil
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
		`UPDATE media
		 SET status = ?, updated_at = ?
		 WHERE id = ?`,
		status,
		nowUTC.Format(time.RFC3339),
		id,
	)
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s rows affected: %w", action, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("%s: media %d not found", action, id)
	}

	return nil
}

func applyPreviewFields(
	item *media.Media,
	previewVideoPath sql.NullString,
	previewVideoSizeBytes sql.NullInt64,
	previewVideoMIMEType sql.NullString,
	previewVideoCreatedAt sql.NullString,
) {
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
		if parsed, err := time.Parse(time.RFC3339, previewVideoCreatedAt.String); err == nil {
			item.PreviewVideoCreatedAtUTC = &parsed
		}
	}
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

func formatOptionalTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC().Format(time.RFC3339)
}
