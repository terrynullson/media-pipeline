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
			media_id, type, payload, status, attempts, error_message, created_at, updated_at
		 ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		j.MediaID,
		j.Type,
		j.Payload,
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

func (r *JobRepository) ExistsActiveOrDone(ctx context.Context, mediaID int64, jobType job.Type) (bool, error) {
	var exists int
	err := r.db.QueryRowContext(
		ctx,
		`SELECT 1
		 FROM jobs
		 WHERE media_id = ?
		   AND type = ?
		   AND status IN (?, ?, ?)
		 LIMIT 1`,
		mediaID,
		jobType,
		job.StatusPending,
		job.StatusRunning,
		job.StatusDone,
	).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}

	return false, fmt.Errorf("check existing job: %w", err)
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
		 RETURNING id, media_id, type, payload, status, attempts, error_message, created_at, updated_at`,
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
		&claimed.Payload,
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

func (r *JobRepository) ListByStatus(ctx context.Context, jobType job.Type, status job.Status) ([]job.Job, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, media_id, type, payload, status, attempts, error_message, created_at, updated_at
		 FROM jobs
		 WHERE type = ? AND status = ?
		 ORDER BY datetime(created_at) ASC, id ASC`,
		jobType,
		status,
	)
	if err != nil {
		return nil, fmt.Errorf("list jobs by status: %w", err)
	}
	defer rows.Close()

	items := make([]job.Job, 0)
	for rows.Next() {
		var item job.Job
		var createdAt string
		var updatedAt string
		if err := rows.Scan(
			&item.ID,
			&item.MediaID,
			&item.Type,
			&item.Payload,
			&item.Status,
			&item.Attempts,
			&item.ErrorMessage,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan job row: %w", err)
		}

		item.CreatedAtUTC, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse job created_at: %w", err)
		}
		item.UpdatedAtUTC, err = time.Parse(time.RFC3339, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse job updated_at: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs by status: %w", err)
	}

	return items, nil
}

func (r *JobRepository) Requeue(ctx context.Context, id int64, errorMessage string, nowUTC time.Time) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE jobs
		 SET status = ?, error_message = ?, updated_at = ?
		 WHERE id = ?`,
		job.StatusPending,
		errorMessage,
		nowUTC.Format(time.RFC3339),
		id,
	)
	if err != nil {
		return fmt.Errorf("requeue job: %w", err)
	}

	return ensureRowsAffected(result, id, "requeue job")
}

func (r *JobRepository) FindLatestByMediaAndType(ctx context.Context, mediaID int64, jobType job.Type) (job.Job, bool, error) {
	row := r.db.QueryRowContext(
		ctx,
		`SELECT id, media_id, type, payload, status, attempts, error_message, created_at, updated_at
		 FROM jobs
		 WHERE media_id = ? AND type = ?
		 ORDER BY datetime(created_at) DESC, id DESC
		 LIMIT 1`,
		mediaID,
		jobType,
	)

	var item job.Job
	var createdAt string
	var updatedAt string
	if err := row.Scan(
		&item.ID,
		&item.MediaID,
		&item.Type,
		&item.Payload,
		&item.Status,
		&item.Attempts,
		&item.ErrorMessage,
		&createdAt,
		&updatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return job.Job{}, false, nil
		}
		return job.Job{}, false, fmt.Errorf("scan latest job by media id %d and type %s: %w", mediaID, jobType, err)
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return job.Job{}, false, fmt.Errorf("parse latest job created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return job.Job{}, false, fmt.Errorf("parse latest job updated_at: %w", err)
	}
	item.CreatedAtUTC = parsedCreatedAt
	item.UpdatedAtUTC = parsedUpdatedAt

	return item, true, nil
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
