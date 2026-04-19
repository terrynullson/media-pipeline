package repositories

import (
	"context"
	"database/sql"
	"errors"
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

// Save upserts the transcript and replaces all of its segments. When the
// parent media has a known recording_started_at, segment_started_at /
// segment_ended_at are derived from it (start_sec / end_sec measured from the
// recording origin). Otherwise they are NULL.
func (r *TranscriptRepository) Save(ctx context.Context, item transcript.Transcript) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transcript tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Upsert by media_id (UNIQUE (media_id) on transcripts).
	var transcriptID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO transcripts (media_id, language, full_text, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (media_id) DO UPDATE
		   SET language   = EXCLUDED.language,
		       full_text  = EXCLUDED.full_text,
		       updated_at = EXCLUDED.updated_at
		 RETURNING id`,
		item.MediaID,
		item.Language,
		item.FullText,
		item.CreatedAtUTC.UTC(),
		item.UpdatedAtUTC.UTC(),
	).Scan(&transcriptID)
	if err != nil {
		return fmt.Errorf("upsert transcript: %w", err)
	}

	// Look up the parent recording origin (may be NULL — non-broadcast media).
	var recordingStart sql.NullTime
	if err := tx.QueryRowContext(ctx,
		`SELECT recording_started_at FROM media WHERE id = $1`, item.MediaID,
	).Scan(&recordingStart); err != nil {
		return fmt.Errorf("load recording_started_at for media %d: %w", item.MediaID, err)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM transcript_segments WHERE transcript_id = $1`, transcriptID,
	); err != nil {
		return fmt.Errorf("delete transcript segments: %w", err)
	}

	for index, segment := range item.Segments {
		var segStart, segEnd any
		if recordingStart.Valid {
			segStart = recordingStart.Time.Add(secondsToDuration(segment.StartSec)).UTC()
			segEnd = recordingStart.Time.Add(secondsToDuration(segment.EndSec)).UTC()
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO transcript_segments (
				transcript_id, segment_index, start_sec, end_sec, text, confidence,
				segment_started_at, segment_ended_at, created_at
			 ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			transcriptID,
			index,
			segment.StartSec,
			segment.EndSec,
			segment.Text,
			nullableFloat64(segment.Confidence),
			segStart,
			segEnd,
			item.UpdatedAtUTC.UTC(),
		); err != nil {
			return fmt.Errorf("insert transcript segment %d: %w", index, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transcript tx: %w", err)
	}

	return nil
}

func (r *TranscriptRepository) ExistsByMediaID(ctx context.Context, mediaID int64) (bool, error) {
	var exists int
	err := r.db.QueryRowContext(
		ctx,
		`SELECT 1 FROM transcripts WHERE media_id = $1 LIMIT 1`,
		mediaID,
	).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, fmt.Errorf("check transcript exists for media %d: %w", mediaID, err)
}

// ListRecentWithSegments returns the most-recent `limit` transcripts with their segments,
// ordered newest-first. Used for trigger rule preview.
func (r *TranscriptRepository) ListRecentWithSegments(ctx context.Context, limit int) ([]transcript.Transcript, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, media_id, language, full_text, created_at, updated_at
		 FROM transcripts
		 ORDER BY created_at DESC, id DESC
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list recent transcripts: %w", err)
	}
	defer rows.Close()

	items := make([]transcript.Transcript, 0)
	for rows.Next() {
		var item transcript.Transcript
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&item.ID, &item.MediaID, &item.Language, &item.FullText, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan transcript: %w", err)
		}
		item.CreatedAtUTC = createdAt.UTC()
		item.UpdatedAtUTC = updatedAt.UTC()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent transcripts: %w", err)
	}

	// Load segments for each transcript.
	for i := range items {
		segments, err := r.loadSegments(ctx, items[i].ID)
		if err != nil {
			return nil, err
		}
		items[i].Segments = segments
	}

	return items, nil
}

func (r *TranscriptRepository) GetByMediaID(ctx context.Context, mediaID int64) (transcript.Transcript, bool, error) {
	row := r.db.QueryRowContext(
		ctx,
		`SELECT id, media_id, language, full_text, created_at, updated_at
		 FROM transcripts
		 WHERE media_id = $1`,
		mediaID,
	)

	var item transcript.Transcript
	var createdAt, updatedAt time.Time
	if err := row.Scan(
		&item.ID,
		&item.MediaID,
		&item.Language,
		&item.FullText,
		&createdAt,
		&updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return transcript.Transcript{}, false, nil
		}
		return transcript.Transcript{}, false, fmt.Errorf("scan transcript by media id %d: %w", mediaID, err)
	}

	item.CreatedAtUTC = createdAt.UTC()
	item.UpdatedAtUTC = updatedAt.UTC()

	segments, err := r.loadSegments(ctx, item.ID)
	if err != nil {
		return transcript.Transcript{}, false, err
	}
	item.Segments = segments

	return item, true, nil
}

func (r *TranscriptRepository) loadSegments(ctx context.Context, transcriptID int64) ([]transcript.Segment, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT start_sec, end_sec, text, confidence, segment_started_at, segment_ended_at
		 FROM transcript_segments
		 WHERE transcript_id = $1
		 ORDER BY segment_index ASC, id ASC`,
		transcriptID,
	)
	if err != nil {
		return nil, fmt.Errorf("query transcript segments by transcript id %d: %w", transcriptID, err)
	}
	defer rows.Close()

	segments := make([]transcript.Segment, 0)
	for rows.Next() {
		var seg transcript.Segment
		var confidence sql.NullFloat64
		var segStart, segEnd sql.NullTime
		if err := rows.Scan(
			&seg.StartSec,
			&seg.EndSec,
			&seg.Text,
			&confidence,
			&segStart,
			&segEnd,
		); err != nil {
			return nil, fmt.Errorf("scan transcript segment: %w", err)
		}
		if confidence.Valid {
			v := confidence.Float64
			seg.Confidence = &v
		}
		if segStart.Valid {
			t := segStart.Time.UTC()
			seg.StartedAtUTC = &t
		}
		if segEnd.Valid {
			t := segEnd.Time.UTC()
			seg.EndedAtUTC = &t
		}
		segments = append(segments, seg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate transcript segments: %w", err)
	}

	return segments, nil
}

// secondsToDuration converts a fractional seconds value (as emitted by
// faster-whisper) into a time.Duration without losing sub-second precision.
func secondsToDuration(seconds float64) time.Duration {
	return time.Duration(seconds * float64(time.Second))
}
