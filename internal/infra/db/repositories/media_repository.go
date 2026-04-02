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
			size_bytes, storage_path, extracted_audio_path, transcript_text, status, created_at, updated_at
		 ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.OriginalName,
		m.StoredName,
		m.Extension,
		m.MIMEType,
		m.SizeBytes,
		m.StoragePath,
		m.ExtractedAudioPath,
		m.TranscriptText,
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

func (r *MediaRepository) ListRecent(ctx context.Context, limit int) ([]media.Media, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, original_name, stored_name, extension, mime_type,
			size_bytes, storage_path, extracted_audio_path, transcript_text, status, created_at, updated_at
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
		if scanErr := rows.Scan(
			&item.ID,
			&item.OriginalName,
			&item.StoredName,
			&item.Extension,
			&item.MIMEType,
			&item.SizeBytes,
			&item.StoragePath,
			&item.ExtractedAudioPath,
			&item.TranscriptText,
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
			size_bytes, storage_path, extracted_audio_path, transcript_text, status, created_at, updated_at
		 FROM media
		 WHERE id = ?`,
		id,
	)

	var item media.Media
	var createdAt string
	var updatedAt string
	if err := row.Scan(
		&item.ID,
		&item.OriginalName,
		&item.StoredName,
		&item.Extension,
		&item.MIMEType,
		&item.SizeBytes,
		&item.StoragePath,
		&item.ExtractedAudioPath,
		&item.TranscriptText,
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
