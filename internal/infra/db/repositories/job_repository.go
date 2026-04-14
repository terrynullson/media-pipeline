package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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
			media_id, type, payload, status, attempts, error_message, created_at, updated_at,
			started_at, finished_at, duration_ms, progress_percent, progress_label, progress_is_estimate, progress_updated_at
		 ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.MediaID,
		j.Type,
		j.Payload,
		j.Status,
		j.Attempts,
		j.ErrorMessage,
		j.CreatedAtUTC.Format(time.RFC3339),
		j.UpdatedAtUTC.Format(time.RFC3339),
		nullableTimeString(j.StartedAtUTC),
		nullableTimeString(j.FinishedAtUTC),
		nullableInt64(j.DurationMS),
		nullableFloat64(j.ProgressPercent),
		j.ProgressLabel,
		boolToIntJob(j.ProgressIsEstimated),
		nullableTimeString(j.ProgressUpdatedAtUTC),
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
		 SET status = ?, error_message = '', updated_at = ?, started_at = ?, finished_at = NULL, duration_ms = NULL,
		     progress_percent = NULL, progress_label = '', progress_is_estimate = 0, progress_updated_at = NULL
		 WHERE id = (
		 	SELECT id
		 	FROM jobs
		 	WHERE status = ? AND type = ?
		 	ORDER BY datetime(created_at) ASC, id ASC
		 	LIMIT 1
		 )
		 RETURNING id, media_id, type, payload, status, attempts, error_message, created_at, updated_at,
		           started_at, finished_at, duration_ms, progress_percent, progress_label, progress_is_estimate, progress_updated_at`,
		job.StatusRunning,
		nowUTC.Format(time.RFC3339),
		nowUTC.Format(time.RFC3339),
		job.StatusPending,
		jobType,
	)

	claimed, ok, err := scanJobRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return job.Job{}, false, nil
		}
		return job.Job{}, false, fmt.Errorf("claim next pending job: %w", err)
	}

	return claimed, ok, nil
}

func (r *JobRepository) MarkDone(ctx context.Context, id int64, nowUTC time.Time) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE jobs
		 SET status = ?, error_message = '', updated_at = ?, finished_at = ?,
		     duration_ms = CAST((julianday(?) - julianday(started_at)) * 86400000 AS INTEGER)
		 WHERE id = ?`,
		job.StatusDone,
		nowUTC.Format(time.RFC3339),
		nowUTC.Format(time.RFC3339),
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
		 SET status = ?, attempts = attempts + 1, error_message = ?, updated_at = ?, finished_at = ?,
		     duration_ms = CAST((julianday(?) - julianday(started_at)) * 86400000 AS INTEGER)
		 WHERE id = ?`,
		job.StatusFailed,
		errorMessage,
		nowUTC.Format(time.RFC3339),
		nowUTC.Format(time.RFC3339),
		nowUTC.Format(time.RFC3339),
		id,
	)
	if err != nil {
		return fmt.Errorf("mark job failed: %w", err)
	}

	return ensureRowsAffected(result, id, "mark job failed")
}

func (r *JobRepository) UpdateProgress(
	ctx context.Context,
	id int64,
	progressPercent *float64,
	progressLabel string,
	isEstimate bool,
	nowUTC time.Time,
) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE jobs
		 SET progress_percent = ?, progress_label = ?, progress_is_estimate = ?, progress_updated_at = ?, updated_at = ?
		 WHERE id = ?`,
		nullableFloat64(progressPercent),
		progressLabel,
		boolToIntJob(isEstimate),
		nowUTC.Format(time.RFC3339),
		nowUTC.Format(time.RFC3339),
		id,
	)
	if err != nil {
		return fmt.Errorf("update job progress: %w", err)
	}

	return ensureRowsAffected(result, id, "update job progress")
}

func (r *JobRepository) ListByStatus(ctx context.Context, jobType job.Type, status job.Status) ([]job.Job, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, media_id, type, payload, status, attempts, error_message, created_at, updated_at,
		        started_at, finished_at, duration_ms, progress_percent, progress_label, progress_is_estimate, progress_updated_at
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
		item, err := scanJobRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan job row: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs by status: %w", err)
	}

	return items, nil
}

func (r *JobRepository) ListByMediaID(ctx context.Context, mediaID int64) ([]job.Job, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, media_id, type, payload, status, attempts, error_message, created_at, updated_at,
		        started_at, finished_at, duration_ms, progress_percent, progress_label, progress_is_estimate, progress_updated_at
		 FROM jobs
		 WHERE media_id = ?
		 ORDER BY datetime(created_at) DESC, id DESC`,
		mediaID,
	)
	if err != nil {
		return nil, fmt.Errorf("list jobs by media id: %w", err)
	}
	defer rows.Close()

	items := make([]job.Job, 0)
	for rows.Next() {
		item, err := scanJobRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan job row by media id: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs by media id: %w", err)
	}

	return items, nil
}

func (r *JobRepository) ListAllByStatus(ctx context.Context, status job.Status) ([]job.Job, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, media_id, type, payload, status, attempts, error_message, created_at, updated_at,
		        started_at, finished_at, duration_ms, progress_percent, progress_label, progress_is_estimate, progress_updated_at
		 FROM jobs
		 WHERE status = ?
		 ORDER BY datetime(created_at) ASC, id ASC`,
		status,
	)
	if err != nil {
		return nil, fmt.Errorf("list all jobs by status: %w", err)
	}
	defer rows.Close()

	items := make([]job.Job, 0)
	for rows.Next() {
		item, err := scanJobRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan job row: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate all jobs by status: %w", err)
	}

	return items, nil
}

func (r *JobRepository) ListPendingCoreJobsWithMediaAge(ctx context.Context, jobTypes []job.Type) ([]job.JobWithMediaAge, error) {
	if len(jobTypes) == 0 {
		return nil, nil
	}

	// Build the IN clause placeholders dynamically.
	placeholders := make([]any, len(jobTypes))
	for i, t := range jobTypes {
		placeholders[i] = t
	}
	inClause := "?" + strings.Repeat(",?", len(jobTypes)-1)

	query := `SELECT j.id, j.media_id, j.type, j.payload, j.status, j.attempts, j.error_message,
	                 j.created_at, j.updated_at, j.started_at, j.finished_at, j.duration_ms,
	                 j.progress_percent, j.progress_label, j.progress_is_estimate, j.progress_updated_at,
	                 m.created_at AS media_created_at
	          FROM jobs j
	          JOIN media m ON m.id = j.media_id
	          WHERE j.status = ? AND j.type IN (` + inClause + `)
	          ORDER BY datetime(m.created_at) ASC, j.id ASC`

	args := make([]any, 0, 1+len(jobTypes))
	args = append(args, job.StatusPending)
	args = append(args, placeholders...)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list pending core jobs with media age: %w", err)
	}
	defer rows.Close()

	var items []job.JobWithMediaAge
	for rows.Next() {
		var j job.Job
		var createdAt, updatedAt, mediaCreatedAt string
		var startedAt, finishedAt, progressUpdatedAt sql.NullString
		var durationMS sql.NullInt64
		var progressPercent sql.NullFloat64
		var progressLabel string
		var progressIsEstimate int

		if err := rows.Scan(
			&j.ID, &j.MediaID, &j.Type, &j.Payload, &j.Status, &j.Attempts, &j.ErrorMessage,
			&createdAt, &updatedAt, &startedAt, &finishedAt, &durationMS,
			&progressPercent, &progressLabel, &progressIsEstimate, &progressUpdatedAt,
			&mediaCreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan pending core job: %w", err)
		}

		if parsed, err := time.Parse(time.RFC3339, createdAt); err == nil {
			j.CreatedAtUTC = parsed
		}
		if parsed, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			j.UpdatedAtUTC = parsed
		}
		applyOptionalJobFields(&j, startedAt, finishedAt, durationMS, progressPercent, progressLabel, progressIsEstimate, progressUpdatedAt)

		var mediaAge time.Time
		if parsed, err := time.Parse(time.RFC3339, mediaCreatedAt); err == nil {
			mediaAge = parsed
		}

		items = append(items, job.JobWithMediaAge{Job: j, MediaCreatedAtUTC: mediaAge})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending core jobs: %w", err)
	}

	return items, nil
}

func (r *JobRepository) Requeue(ctx context.Context, id int64, errorMessage string, nowUTC time.Time) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE jobs
		 SET status = ?, error_message = ?, updated_at = ?, started_at = NULL, finished_at = NULL, duration_ms = NULL,
		     progress_percent = NULL, progress_label = '', progress_is_estimate = 0, progress_updated_at = NULL
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
		`SELECT id, media_id, type, payload, status, attempts, error_message, created_at, updated_at,
		        started_at, finished_at, duration_ms, progress_percent, progress_label, progress_is_estimate, progress_updated_at
		 FROM jobs
		 WHERE media_id = ? AND type = ?
		 ORDER BY datetime(created_at) DESC, id DESC
		 LIMIT 1`,
		mediaID,
		jobType,
	)

	item, ok, err := scanJobRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return job.Job{}, false, nil
		}
		return job.Job{}, false, fmt.Errorf("scan latest job by media id %d and type %s: %w", mediaID, jobType, err)
	}

	return item, ok, nil
}

func scanJobRow(row *sql.Row) (job.Job, bool, error) {
	var item job.Job
	var createdAt string
	var updatedAt string
	var startedAt sql.NullString
	var finishedAt sql.NullString
	var durationMS sql.NullInt64
	var progressPercent sql.NullFloat64
	var progressLabel string
	var progressIsEstimate int
	var progressUpdatedAt sql.NullString
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
		&startedAt,
		&finishedAt,
		&durationMS,
		&progressPercent,
		&progressLabel,
		&progressIsEstimate,
		&progressUpdatedAt,
	); err != nil {
		return job.Job{}, false, err
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return job.Job{}, false, fmt.Errorf("parse job created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return job.Job{}, false, fmt.Errorf("parse job updated_at: %w", err)
	}
	item.CreatedAtUTC = parsedCreatedAt
	item.UpdatedAtUTC = parsedUpdatedAt
	applyOptionalJobFields(&item, startedAt, finishedAt, durationMS, progressPercent, progressLabel, progressIsEstimate, progressUpdatedAt)

	return item, true, nil
}

func scanJobRows(rows *sql.Rows) (job.Job, error) {
	var item job.Job
	var createdAt string
	var updatedAt string
	var startedAt sql.NullString
	var finishedAt sql.NullString
	var durationMS sql.NullInt64
	var progressPercent sql.NullFloat64
	var progressLabel string
	var progressIsEstimate int
	var progressUpdatedAt sql.NullString
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
		&startedAt,
		&finishedAt,
		&durationMS,
		&progressPercent,
		&progressLabel,
		&progressIsEstimate,
		&progressUpdatedAt,
	); err != nil {
		return job.Job{}, err
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return job.Job{}, fmt.Errorf("parse job created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return job.Job{}, fmt.Errorf("parse job updated_at: %w", err)
	}
	item.CreatedAtUTC = parsedCreatedAt
	item.UpdatedAtUTC = parsedUpdatedAt
	applyOptionalJobFields(&item, startedAt, finishedAt, durationMS, progressPercent, progressLabel, progressIsEstimate, progressUpdatedAt)

	return item, nil
}

func applyOptionalJobFields(
	item *job.Job,
	startedAt sql.NullString,
	finishedAt sql.NullString,
	durationMS sql.NullInt64,
	progressPercent sql.NullFloat64,
	progressLabel string,
	progressIsEstimate int,
	progressUpdatedAt sql.NullString,
) {
	item.ProgressLabel = progressLabel
	item.ProgressIsEstimated = progressIsEstimate == 1

	if startedAt.Valid {
		if parsed, err := time.Parse(time.RFC3339, startedAt.String); err == nil {
			item.StartedAtUTC = &parsed
		}
	}
	if finishedAt.Valid {
		if parsed, err := time.Parse(time.RFC3339, finishedAt.String); err == nil {
			item.FinishedAtUTC = &parsed
		}
	}
	if durationMS.Valid {
		value := durationMS.Int64
		item.DurationMS = &value
	}
	if progressPercent.Valid {
		value := progressPercent.Float64
		item.ProgressPercent = &value
	}
	if progressUpdatedAt.Valid {
		if parsed, err := time.Parse(time.RFC3339, progressUpdatedAt.String); err == nil {
			item.ProgressUpdatedAtUTC = &parsed
		}
	}
}

func nullableTimeString(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}

	return value.UTC().Format(time.RFC3339)
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}

	return *value
}

func nullableFloat64(value *float64) any {
	if value == nil {
		return nil
	}

	return *value
}

func boolToIntJob(value bool) int {
	if value {
		return 1
	}

	return 0
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
