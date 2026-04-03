package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"media-pipeline/internal/domain/transcript"
)

type TranscriptRepository struct {
	db *sql.DB
}

func NewTranscriptRepository(db *sql.DB) *TranscriptRepository {
	return &TranscriptRepository{db: db}
}

func (r *TranscriptRepository) Save(ctx context.Context, item transcript.Transcript) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transcript tx: %w", err)
	}
	defer tx.Rollback()

	var transcriptID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM transcripts WHERE media_id = ?", item.MediaID).Scan(&transcriptID)
	switch {
	case err == nil:
		if _, err := tx.ExecContext(
			ctx,
			`UPDATE transcripts
			 SET language = ?, full_text = ?, updated_at = ?
			 WHERE id = ?`,
			item.Language,
			item.FullText,
			item.UpdatedAtUTC.Format(time.RFC3339),
			transcriptID,
		); err != nil {
			return fmt.Errorf("update transcript: %w", err)
		}
	case err == sql.ErrNoRows:
		result, execErr := tx.ExecContext(
			ctx,
			`INSERT INTO transcripts (media_id, language, full_text, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?)`,
			item.MediaID,
			item.Language,
			item.FullText,
			item.CreatedAtUTC.Format(time.RFC3339),
			item.UpdatedAtUTC.Format(time.RFC3339),
		)
		if execErr != nil {
			return fmt.Errorf("insert transcript: %w", execErr)
		}
		transcriptID, err = result.LastInsertId()
		if err != nil {
			return fmt.Errorf("transcript last insert id: %w", err)
		}
	default:
		return fmt.Errorf("load transcript by media id: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM transcript_segments WHERE transcript_id = ?", transcriptID); err != nil {
		return fmt.Errorf("delete transcript segments: %w", err)
	}

	for index, segment := range item.Segments {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO transcript_segments (
				transcript_id, segment_index, start_sec, end_sec, text, confidence, created_at
			 ) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			transcriptID,
			index,
			segment.StartSec,
			segment.EndSec,
			segment.Text,
			segment.Confidence,
			item.UpdatedAtUTC.Format(time.RFC3339),
		); err != nil {
			return fmt.Errorf("insert transcript segment %d: %w", index, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transcript tx: %w", err)
	}

	return nil
}

func (r *TranscriptRepository) GetByMediaID(ctx context.Context, mediaID int64) (transcript.Transcript, bool, error) {
	row := r.db.QueryRowContext(
		ctx,
		`SELECT id, media_id, language, full_text, created_at, updated_at
		 FROM transcripts
		 WHERE media_id = ?`,
		mediaID,
	)

	var item transcript.Transcript
	var createdAt string
	var updatedAt string
	if err := row.Scan(
		&item.ID,
		&item.MediaID,
		&item.Language,
		&item.FullText,
		&createdAt,
		&updatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return transcript.Transcript{}, false, nil
		}
		return transcript.Transcript{}, false, fmt.Errorf("scan transcript by media id %d: %w", mediaID, err)
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return transcript.Transcript{}, false, fmt.Errorf("parse transcript created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return transcript.Transcript{}, false, fmt.Errorf("parse transcript updated_at: %w", err)
	}
	item.CreatedAtUTC = parsedCreatedAt
	item.UpdatedAtUTC = parsedUpdatedAt

	rows, err := r.db.QueryContext(
		ctx,
		`SELECT start_sec, end_sec, text, confidence
		 FROM transcript_segments
		 WHERE transcript_id = ?
		 ORDER BY segment_index ASC, id ASC`,
		item.ID,
	)
	if err != nil {
		return transcript.Transcript{}, false, fmt.Errorf("query transcript segments by transcript id %d: %w", item.ID, err)
	}
	defer rows.Close()

	item.Segments = make([]transcript.Segment, 0)
	for rows.Next() {
		var segment transcript.Segment
		if err := rows.Scan(
			&segment.StartSec,
			&segment.EndSec,
			&segment.Text,
			&segment.Confidence,
		); err != nil {
			return transcript.Transcript{}, false, fmt.Errorf("scan transcript segment: %w", err)
		}
		item.Segments = append(item.Segments, segment)
	}
	if err := rows.Err(); err != nil {
		return transcript.Transcript{}, false, fmt.Errorf("iterate transcript segments: %w", err)
	}

	return item, true, nil
}
