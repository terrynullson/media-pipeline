package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	appworker "media-pipeline/internal/app/worker"
	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/media"
)

// TranscriptionCommitRepository implements appworker.TranscriptionCommitter.
// It executes the four steps that complete a transcription job — save transcript,
// transition media status, enqueue the next stage, mark job done — inside a
// single SQLite transaction so a crash between steps can never leave the
// pipeline in a partially-committed state.
type TranscriptionCommitRepository struct {
	db *sql.DB
}

func NewTranscriptionCommitRepository(db *sql.DB) *TranscriptionCommitRepository {
	return &TranscriptionCommitRepository{db: db}
}

// CommitTranscribeJob runs all four completion steps atomically.
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
	defer tx.Rollback() //nolint:errcheck // superseded by Commit

	// ── 1. Upsert transcript header ─────────────────────────────────────────
	var transcriptID int64
	err = tx.QueryRowContext(
		ctx,
		"SELECT id FROM transcripts WHERE media_id = ?",
		in.MediaID,
	).Scan(&transcriptID)

	switch {
	case err == nil:
		// existing row — update in place
		if _, err := tx.ExecContext(
			ctx,
			`UPDATE transcripts SET language = ?, full_text = ?, updated_at = ? WHERE id = ?`,
			in.Transcript.Language,
			in.Transcript.FullText,
			in.NowUTC.Format(time.RFC3339),
			transcriptID,
		); err != nil {
			return fmt.Errorf("update transcript: %w", err)
		}

	case err == sql.ErrNoRows:
		res, execErr := tx.ExecContext(
			ctx,
			`INSERT INTO transcripts (media_id, language, full_text, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?)`,
			in.MediaID,
			in.Transcript.Language,
			in.Transcript.FullText,
			in.NowUTC.Format(time.RFC3339),
			in.NowUTC.Format(time.RFC3339),
		)
		if execErr != nil {
			return fmt.Errorf("insert transcript: %w", execErr)
		}
		if transcriptID, err = res.LastInsertId(); err != nil {
			return fmt.Errorf("transcript last insert id: %w", err)
		}

	default:
		return fmt.Errorf("load transcript for media %d: %w", in.MediaID, err)
	}

	// ── 2. Replace transcript segments ──────────────────────────────────────
	if _, err := tx.ExecContext(
		ctx,
		"DELETE FROM transcript_segments WHERE transcript_id = ?",
		transcriptID,
	); err != nil {
		return fmt.Errorf("delete old transcript segments: %w", err)
	}

	for i, seg := range in.Transcript.Segments {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO transcript_segments
			   (transcript_id, segment_index, start_sec, end_sec, text, confidence, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			transcriptID, i, seg.StartSec, seg.EndSec, seg.Text, seg.Confidence,
			in.NowUTC.Format(time.RFC3339),
		); err != nil {
			return fmt.Errorf("insert transcript segment %d: %w", i, err)
		}
	}

	// ── 3. Mark media as transcribed ────────────────────────────────────────
	res, err := tx.ExecContext(
		ctx,
		`UPDATE media SET status = ?, transcript_text = ?, updated_at = ? WHERE id = ?`,
		media.StatusTranscribed,
		in.Transcript.FullText,
		in.NowUTC.Format(time.RFC3339),
		in.MediaID,
	)
	if err != nil {
		return fmt.Errorf("mark media transcribed: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return fmt.Errorf("mark media transcribed: media %d not found", in.MediaID)
	}

	// ── 4. Enqueue next pipeline stage (idempotent) ─────────────────────────
	var jobExists int
	err = tx.QueryRowContext(
		ctx,
		`SELECT 1 FROM jobs
		 WHERE media_id = ? AND type = ? AND status IN (?, ?, ?)
		 LIMIT 1`,
		in.MediaID, in.NextJobType,
		job.StatusPending, job.StatusRunning, job.StatusDone,
	).Scan(&jobExists)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("check next job existence: %w", err)
	}
	if err == sql.ErrNoRows {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO jobs
			   (media_id, type, payload, status, attempts, error_message, created_at, updated_at,
			    started_at, finished_at, duration_ms,
			    progress_percent, progress_label, progress_is_estimate, progress_updated_at)
			 VALUES (?, ?, '', ?, 0, '', ?, ?, NULL, NULL, NULL, NULL, '', 0, NULL)`,
			in.MediaID, in.NextJobType,
			job.StatusPending,
			in.NowUTC.Format(time.RFC3339),
			in.NowUTC.Format(time.RFC3339),
		); err != nil {
			return fmt.Errorf("create next job (%s): %w", in.NextJobType, err)
		}
	}

	// ── 5. Mark transcription job done ──────────────────────────────────────
	res, err = tx.ExecContext(
		ctx,
		`UPDATE jobs
		 SET status = ?, error_message = '', updated_at = ?, finished_at = ?,
		     duration_ms = CAST((julianday(?) - julianday(started_at)) * 86400000 AS INTEGER)
		 WHERE id = ?`,
		job.StatusDone,
		in.NowUTC.Format(time.RFC3339),
		in.NowUTC.Format(time.RFC3339),
		in.NowUTC.Format(time.RFC3339),
		in.JobID,
	)
	if err != nil {
		return fmt.Errorf("mark job done: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return fmt.Errorf("mark job done: job %d not found", in.JobID)
	}

	return tx.Commit()
}
