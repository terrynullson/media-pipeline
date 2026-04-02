package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"media-pipeline/internal/domain/job"
)

type JobRepository struct {
	db *sql.DB
}

func NewJobRepository(db *sql.DB) *JobRepository {
	return &JobRepository{db: db}
}

func (r *JobRepository) Create(ctx context.Context, j job.Job) (int64, error) {
	result, err := r.db.ExecContext(
		ctx,
		`INSERT INTO jobs (
			media_id, type, status, attempts, error_message, created_at, updated_at
		 ) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		j.MediaID,
		j.Type,
		j.Status,
		j.Attempts,
		j.ErrorMessage,
		j.CreatedAtUTC.Format(time.RFC3339),
		j.UpdatedAtUTC.Format(time.RFC3339),
	)
	if err != nil {
		return 0, fmt.Errorf("insert job: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("job last insert id: %w", err)
	}

	return id, nil
}

func (r *JobRepository) ClaimNextPending(ctx context.Context, jobType job.Type, nowUTC time.Time) (job.Job, bool, error) {
	row := r.db.QueryRowContext(
		ctx,
		`UPDATE jobs
		 SET status = ?, error_message = '', updated_at = ?
		 WHERE id = (
		 	SELECT id
		 	FROM jobs
		 	WHERE status = ? AND type = ?
		 	ORDER BY datetime(created_at) ASC, id ASC
		 	LIMIT 1
		 )
		 RETURNING id, media_id, type, status, attempts, error_message, created_at, updated_at`,
		job.StatusRunning,
		nowUTC.Format(time.RFC3339),
		job.StatusPending,
		jobType,
	)

	var claimed job.Job
	var createdAt string
	var updatedAt string
	err := row.Scan(
		&claimed.ID,
		&claimed.MediaID,
		&claimed.Type,
		&claimed.Status,
		&claimed.Attempts,
		&claimed.ErrorMessage,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return job.Job{}, false, nil
		}
		return job.Job{}, false, fmt.Errorf("claim next pending job: %w", err)
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return job.Job{}, false, fmt.Errorf("parse claimed job created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return job.Job{}, false, fmt.Errorf("parse claimed job updated_at: %w", err)
	}
	claimed.CreatedAtUTC = parsedCreatedAt
	claimed.UpdatedAtUTC = parsedUpdatedAt

	return claimed, true, nil
}

func (r *JobRepository) MarkDone(ctx context.Context, id int64, nowUTC time.Time) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE jobs
		 SET status = ?, error_message = '', updated_at = ?
		 WHERE id = ?`,
		job.StatusDone,
		nowUTC.Format(time.RFC3339),
		id,
	)
	if err != nil {
		return fmt.Errorf("mark job done: %w", err)
	}

	return ensureRowsAffected(result, id, "mark job done")
}

func (r *JobRepository) MarkFailed(ctx context.Context, id int64, errorMessage string, nowUTC time.Time) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE jobs
		 SET status = ?, attempts = attempts + 1, error_message = ?, updated_at = ?
		 WHERE id = ?`,
		job.StatusFailed,
		errorMessage,
		nowUTC.Format(time.RFC3339),
		id,
	)
	if err != nil {
		return fmt.Errorf("mark job failed: %w", err)
	}

	return ensureRowsAffected(result, id, "mark job failed")
}

func ensureRowsAffected(result sql.Result, id int64, action string) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s rows affected: %w", action, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("%s: job %d not found", action, id)
	}

	return nil
}
