package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	appworker "media-pipeline/internal/app/worker"
	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/media"
)

// analyticWindowSizeSec is the bucket width (in seconds) used for the
// pre-aggregated transcript_windows rows produced on commit. 10s gives a
// good resolution / row-count tradeoff for analytic UIs over multi-hour
// broadcast recordings.
const analyticWindowSizeSec = 10

// TranscriptionCommitRepository implements appworker.TranscriptionCommitter.
// It executes the completion steps that finalise a transcription job —
// upsert transcript header, replace segments, derive absolute segment times
// from the parent recording, materialise analytic windows, transition media
// status, set recording_ended_at, enqueue the next stage, mark the job done
// — inside a single PostgreSQL transaction so a crash between steps can
// never leave the pipeline in a partially-committed state.
type TranscriptionCommitRepository struct {
	db *sql.DB
}

func NewTranscriptionCommitRepository(db *sql.DB) *TranscriptionCommitRepository {
	return &TranscriptionCommitRepository{db: db}
}

// CommitTranscribeJob runs all completion steps atomically.
// On failure the transaction is rolled back and the error is returned;
// the caller's failJob logic then marks the job failed as usual.
func (r *TranscriptionCommitRepository) CommitTranscribeJob(
	ctx context.Context,
	in appworker.CommitTranscribeInput,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transcription commit tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := in.NowUTC.UTC()

	// ── 1. Upsert transcript header ─────────────────────────────────────────
	var transcriptID int64
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO transcripts (media_id, language, full_text, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $4)
		 ON CONFLICT (media_id) DO UPDATE
		   SET language   = EXCLUDED.language,
		       full_text  = EXCLUDED.full_text,
		       updated_at = EXCLUDED.updated_at
		 RETURNING id`,
		in.MediaID,
		in.Transcript.Language,
		in.Transcript.FullText,
		now,
	).Scan(&transcriptID); err != nil {
		return fmt.Errorf("upsert transcript: %w", err)
	}

	// ── 2. Look up parent recording origin ──────────────────────────────────
	// segment_started_at / segment_ended_at and transcript_windows are only
	// populated when the parent media has a known recording_started_at —
	// otherwise these analytic columns stay NULL and no windows are emitted.
	var recordingStart sql.NullTime
	if err := tx.QueryRowContext(ctx,
		`SELECT recording_started_at FROM media WHERE id = $1`, in.MediaID,
	).Scan(&recordingStart); err != nil {
		return fmt.Errorf("load media recording_started_at: %w", err)
	}

	// ── 3. Replace transcript segments ──────────────────────────────────────
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM transcript_segments WHERE transcript_id = $1`, transcriptID,
	); err != nil {
		return fmt.Errorf("delete old transcript segments: %w", err)
	}

	var maxEndSec float64
	for i, seg := range in.Transcript.Segments {
		var segStart, segEnd any
		if recordingStart.Valid {
			segStart = recordingStart.Time.Add(secondsToDuration(seg.StartSec)).UTC()
			segEnd = recordingStart.Time.Add(secondsToDuration(seg.EndSec)).UTC()
		}
		if seg.EndSec > maxEndSec {
			maxEndSec = seg.EndSec
		}

		var confidence any
		if seg.Confidence != nil {
			confidence = *seg.Confidence
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO transcript_segments
			   (transcript_id, segment_index, start_sec, end_sec, text, confidence,
			    segment_started_at, segment_ended_at, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			transcriptID, i, seg.StartSec, seg.EndSec, seg.Text, confidence,
			segStart, segEnd, now,
		); err != nil {
			return fmt.Errorf("insert transcript segment %d: %w", i, err)
		}
	}

	// ── 4. Materialise analytic windows ─────────────────────────────────────
	// We always wipe + re-insert so a re-transcription replaces the old
	// buckets atomically. Windows are only created when absolute timecodes
	// are available.
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM transcript_windows WHERE transcript_id = $1`, transcriptID,
	); err != nil {
		return fmt.Errorf("delete old transcript windows: %w", err)
	}

	if recordingStart.Valid && len(in.Transcript.Segments) > 0 {
		// Bucket each segment by floor(start_sec / window_size) and aggregate
		// concatenated text + segment count per bucket. window_started_at is
		// recording_started_at + bucket_index * window_size; window_ended_at is
		// the bucket end. Text is joined with a single space separator.
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO transcript_windows
			   (media_id, transcript_id, window_size_sec,
			    window_started_at, window_ended_at,
			    text, segment_count, created_at)
			 SELECT $1, $2, $3,
			        $4::timestamptz + (bucket * $3) * INTERVAL '1 second',
			        $4::timestamptz + ((bucket + 1) * $3) * INTERVAL '1 second',
			        string_agg(text, ' ' ORDER BY segment_index),
			        COUNT(*),
			        $5
			 FROM (
			     SELECT segment_index, text, FLOOR(start_sec / $3)::int AS bucket
			     FROM transcript_segments
			     WHERE transcript_id = $2
			 ) s
			 GROUP BY bucket
			 ORDER BY bucket`,
			in.MediaID, transcriptID, analyticWindowSizeSec,
			recordingStart.Time.UTC(), now,
		); err != nil {
			return fmt.Errorf("insert transcript windows: %w", err)
		}
	}

	// ── 5. Update media: status + transcript text + recording_ended_at ──────
	// recording_ended_at is only set when we have both the origin and at
	// least one segment with an end_sec; otherwise we leave whatever value
	// is already there alone.
	var recordingEnd any
	if recordingStart.Valid && maxEndSec > 0 {
		recordingEnd = recordingStart.Time.Add(secondsToDuration(maxEndSec)).UTC()
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE media
		 SET status = $1,
		     transcript_text = $2,
		     updated_at = $3,
		     recording_ended_at = COALESCE($4, recording_ended_at)
		 WHERE id = $5`,
		media.StatusTranscribed,
		in.Transcript.FullText,
		now,
		recordingEnd,
		in.MediaID,
	)
	if err != nil {
		return fmt.Errorf("mark media transcribed: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return fmt.Errorf("mark media transcribed: media %d not found", in.MediaID)
	}

	// ── 6. Enqueue next pipeline stage (idempotent) ─────────────────────────
	var jobExists int
	err = tx.QueryRowContext(ctx,
		`SELECT 1 FROM jobs
		 WHERE media_id = $1 AND type = $2 AND status IN ($3, $4, $5)
		 LIMIT 1`,
		in.MediaID, in.NextJobType,
		job.StatusPending, job.StatusRunning, job.StatusDone,
	).Scan(&jobExists)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check next job existence: %w", err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO jobs
			   (media_id, type, payload, status, attempts, error_message, created_at, updated_at,
			    started_at, finished_at, duration_ms,
			    progress_percent, progress_label, progress_is_estimate, progress_updated_at)
			 VALUES ($1, $2, '', $3, 0, '', $4, $4, NULL, NULL, NULL, NULL, '', FALSE, NULL)`,
			in.MediaID, in.NextJobType, job.StatusPending, now,
		); err != nil {
			return fmt.Errorf("create next job (%s): %w", in.NextJobType, err)
		}
	}

	// ── 7. Mark transcription job done ──────────────────────────────────────
	res, err = tx.ExecContext(ctx,
		`UPDATE jobs
		 SET status = $1,
		     error_message = '',
		     updated_at = $2,
		     finished_at = $2,
		     duration_ms = CASE
		         WHEN started_at IS NOT NULL
		             THEN CAST(EXTRACT(EPOCH FROM ($2::timestamptz - started_at)) * 1000 AS BIGINT)
		         ELSE duration_ms
		     END
		 WHERE id = $3`,
		job.StatusDone, now, in.JobID,
	)
	if err != nil {
		return fmt.Errorf("mark job done: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return fmt.Errorf("mark job done: job %d not found", in.JobID)
	}

	return tx.Commit()
}
