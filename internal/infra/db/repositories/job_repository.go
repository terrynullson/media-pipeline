package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/media"
)

// jobColumns lists the column projection used by every SELECT/RETURNING in
// this repository. Keeping it in one place makes it harder for the column
// list to drift away from the Scan() target list in scanJobRow / scanJobRows.
const jobColumns = `
	id, media_id, type, payload, status, attempts, error_message,
	created_at, updated_at, started_at, finished_at, duration_ms,
	progress_percent, progress_label, progress_is_estimate, progress_updated_at
`

type JobRepository struct {
	db *sql.DB
}

func NewJobRepository(db *sql.DB) *JobRepository {
	return &JobRepository{db: db}
}

func (r *JobRepository) Create(ctx context.Context, j job.Job) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(
		ctx,
		`INSERT INTO jobs (
			media_id, type, payload, status, attempts, error_message, created_at, updated_at,
			started_at, finished_at, duration_ms, progress_percent, progress_label, progress_is_estimate, progress_updated_at
		 ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		 RETURNING id`,
		j.MediaID,
		j.Type,
		j.Payload,
		j.Status,
		j.Attempts,
		j.ErrorMessage,
		j.CreatedAtUTC.UTC(),
		j.UpdatedAtUTC.UTC(),
		nullableTime(j.StartedAtUTC),
		nullableTime(j.FinishedAtUTC),
		nullableInt64(j.DurationMS),
		nullableFloat64(j.ProgressPercent),
		j.ProgressLabel,
		j.ProgressIsEstimated,
		nullableTime(j.ProgressUpdatedAtUTC),
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert job: %w", err)
	}
	return id, nil
}

func (r *JobRepository) ExistsActiveOrDone(ctx context.Context, mediaID int64, jobType job.Type) (bool, error) {
	var exists int
	err := r.db.QueryRowContext(
		ctx,
		`SELECT 1
		 FROM jobs
		 WHERE media_id = $1
		   AND type = $2
		   AND status IN ($3, $4, $5)
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
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}

	return false, fmt.Errorf("check existing job: %w", err)
}

func (r *JobRepository) ClaimNextPending(ctx context.Context, jobType job.Type, nowUTC time.Time) (job.Job, bool, error) {
	row := r.db.QueryRowContext(
		ctx,
		`UPDATE jobs
		 SET status = $1,
		     error_message = '',
		     updated_at = $2,
		     started_at = $2,
		     finished_at = NULL,
		     duration_ms = NULL,
		     progress_percent = NULL,
		     progress_label = '',
		     progress_is_estimate = FALSE,
		     progress_updated_at = NULL
		 WHERE id = (
		     SELECT id
		     FROM jobs
		     WHERE status = $3 AND type = $4
		     ORDER BY created_at ASC, id ASC
		     FOR UPDATE SKIP LOCKED
		     LIMIT 1
		 )
		 RETURNING `+jobColumns,
		job.StatusRunning,
		nowUTC.UTC(),
		job.StatusPending,
		jobType,
	)

	claimed, ok, err := scanJobRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
		job.StatusDone,
		nowUTC.UTC(),
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
		 SET status = $1,
		     attempts = attempts + 1,
		     error_message = $2,
		     updated_at = $3,
		     finished_at = $3,
		     duration_ms = CASE
		         WHEN started_at IS NOT NULL
		             THEN CAST(EXTRACT(EPOCH FROM ($3::timestamptz - started_at)) * 1000 AS BIGINT)
		         ELSE duration_ms
		     END
		 WHERE id = $4`,
		job.StatusFailed,
		errorMessage,
		nowUTC.UTC(),
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
		 SET progress_percent = $1,
		     progress_label = $2,
		     progress_is_estimate = $3,
		     progress_updated_at = $4,
		     updated_at = $4
		 WHERE id = $5`,
		nullableFloat64(progressPercent),
		progressLabel,
		isEstimate,
		nowUTC.UTC(),
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
		`SELECT `+jobColumns+`
		 FROM jobs
		 WHERE type = $1 AND status = $2
		 ORDER BY created_at ASC, id ASC`,
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
		`SELECT `+jobColumns+`
		 FROM jobs
		 WHERE media_id = $1
		 ORDER BY created_at DESC, id DESC`,
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
		`SELECT `+jobColumns+`
		 FROM jobs
		 WHERE status = $1
		 ORDER BY created_at ASC, id ASC`,
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

	// Build the IN clause placeholders ($2,$3,...) — $1 is reserved for the
	// status filter below.
	inClause, args := buildInClause(2, jobTypes)

	query := `SELECT j.id, j.media_id, j.type, j.payload, j.status, j.attempts, j.error_message,
	                 j.created_at, j.updated_at, j.started_at, j.finished_at, j.duration_ms,
	                 j.progress_percent, j.progress_label, j.progress_is_estimate, j.progress_updated_at,
	                 m.created_at AS media_created_at
	          FROM jobs j
	          JOIN media m ON m.id = j.media_id
	          WHERE j.status = $1 AND j.type IN (` + inClause + `)
	          ORDER BY m.created_at ASC, j.id ASC`

	queryArgs := make([]any, 0, 1+len(args))
	queryArgs = append(queryArgs, job.StatusPending)
	queryArgs = append(queryArgs, args...)

	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("list pending core jobs with media age: %w", err)
	}
	defer rows.Close()

	var items []job.JobWithMediaAge
	for rows.Next() {
		var j job.Job
		var createdAt, updatedAt, mediaCreatedAt time.Time
		var startedAt, finishedAt, progressUpdatedAt sql.NullTime
		var durationMS sql.NullInt64
		var progressPercent sql.NullFloat64
		var progressLabel string
		var progressIsEstimate bool

		if err := rows.Scan(
			&j.ID, &j.MediaID, &j.Type, &j.Payload, &j.Status, &j.Attempts, &j.ErrorMessage,
			&createdAt, &updatedAt, &startedAt, &finishedAt, &durationMS,
			&progressPercent, &progressLabel, &progressIsEstimate, &progressUpdatedAt,
			&mediaCreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan pending core job: %w", err)
		}

		j.CreatedAtUTC = createdAt.UTC()
		j.UpdatedAtUTC = updatedAt.UTC()
		applyOptionalJobFields(&j, startedAt, finishedAt, durationMS, progressPercent, progressLabel, progressIsEstimate, progressUpdatedAt)

		items = append(items, job.JobWithMediaAge{Job: j, MediaCreatedAtUTC: mediaCreatedAt.UTC()})
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
		 SET status = $1,
		     error_message = $2,
		     updated_at = $3,
		     started_at = NULL,
		     finished_at = NULL,
		     duration_ms = NULL,
		     progress_percent = NULL,
		     progress_label = '',
		     progress_is_estimate = FALSE,
		     progress_updated_at = NULL
		 WHERE id = $4`,
		job.StatusPending,
		errorMessage,
		nowUTC.UTC(),
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
		`SELECT `+jobColumns+`
		 FROM jobs
		 WHERE media_id = $1 AND type = $2
		 ORDER BY created_at DESC, id DESC
		 LIMIT 1`,
		mediaID,
		jobType,
	)

	item, ok, err := scanJobRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return job.Job{}, false, nil
		}
		return job.Job{}, false, fmt.Errorf("scan latest job by media id %d and type %s: %w", mediaID, jobType, err)
	}

	return item, ok, nil
}

func (r *JobRepository) ListRecentHistoricalSamples(ctx context.Context, jobTypes []job.Type, limit int) ([]job.HistoricalSample, error) {
	if len(jobTypes) == 0 || limit <= 0 {
		return nil, nil
	}

	// $1 is the status filter; $2..$N+1 are job types; the LIMIT placeholder
	// is appended last.
	inClause, typeArgs := buildInClause(2, jobTypes)

	args := make([]any, 0, len(typeArgs)+2)
	args = append(args, job.StatusDone)
	args = append(args, typeArgs...)
	args = append(args, limit)

	limitPlaceholder := "$" + strconv.Itoa(len(args))

	rows, err := r.db.QueryContext(
		ctx,
		`SELECT j.type, j.media_id, m.size_bytes, m.extension, m.mime_type, j.duration_ms, j.finished_at
		 FROM jobs j
		 JOIN media m ON m.id = j.media_id
		 WHERE j.status = $1
		   AND j.type IN (`+inClause+`)
		   AND j.duration_ms IS NOT NULL
		   AND j.finished_at IS NOT NULL
		 ORDER BY j.finished_at DESC, j.id DESC
		 LIMIT `+limitPlaceholder,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("list recent historical samples: %w", err)
	}
	defer rows.Close()

	items := make([]job.HistoricalSample, 0)
	for rows.Next() {
		var (
			item       job.HistoricalSample
			extension  string
			mimeType   string
			finishedAt time.Time
		)
		if err := rows.Scan(
			&item.JobType,
			&item.MediaID,
			&item.MediaSizeBytes,
			&extension,
			&mimeType,
			&item.DurationMS,
			&finishedAt,
		); err != nil {
			return nil, fmt.Errorf("scan recent historical sample: %w", err)
		}

		item.FinishedAtUTC = finishedAt.UTC()
		item.IsAudioOnly = media.Media{Extension: extension, MIMEType: mimeType}.IsAudioOnly()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent historical samples: %w", err)
	}

	return items, nil
}

// buildInClause renders "$start,$start+1,..." and the matching args slice for
// a typed slice of job types. Used to safely expand IN (...) lists with
// PostgreSQL positional placeholders.
func buildInClause(start int, jobTypes []job.Type) (string, []any) {
	parts := make([]string, len(jobTypes))
	args := make([]any, len(jobTypes))
	for i, t := range jobTypes {
		parts[i] = "$" + strconv.Itoa(start+i)
		args[i] = t
	}
	return strings.Join(parts, ","), args
}

func scanJobRow(row *sql.Row) (job.Job, bool, error) {
	var item job.Job
	var createdAt, updatedAt time.Time
	var startedAt, finishedAt, progressUpdatedAt sql.NullTime
	var durationMS sql.NullInt64
	var progressPercent sql.NullFloat64
	var progressLabel string
	var progressIsEstimate bool
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

	item.CreatedAtUTC = createdAt.UTC()
	item.UpdatedAtUTC = updatedAt.UTC()
	applyOptionalJobFields(&item, startedAt, finishedAt, durationMS, progressPercent, progressLabel, progressIsEstimate, progressUpdatedAt)

	return item, true, nil
}

func scanJobRows(rows *sql.Rows) (job.Job, error) {
	var item job.Job
	var createdAt, updatedAt time.Time
	var startedAt, finishedAt, progressUpdatedAt sql.NullTime
	var durationMS sql.NullInt64
	var progressPercent sql.NullFloat64
	var progressLabel string
	var progressIsEstimate bool
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

	item.CreatedAtUTC = createdAt.UTC()
	item.UpdatedAtUTC = updatedAt.UTC()
	applyOptionalJobFields(&item, startedAt, finishedAt, durationMS, progressPercent, progressLabel, progressIsEstimate, progressUpdatedAt)

	return item, nil
}

func applyOptionalJobFields(
	item *job.Job,
	startedAt sql.NullTime,
	finishedAt sql.NullTime,
	durationMS sql.NullInt64,
	progressPercent sql.NullFloat64,
	progressLabel string,
	progressIsEstimate bool,
	progressUpdatedAt sql.NullTime,
) {
	item.ProgressLabel = progressLabel
	item.ProgressIsEstimated = progressIsEstimate

	if startedAt.Valid {
		t := startedAt.Time.UTC()
		item.StartedAtUTC = &t
	}
	if finishedAt.Valid {
		t := finishedAt.Time.UTC()
		item.FinishedAtUTC = &t
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
		t := progressUpdatedAt.Time.UTC()
		item.ProgressUpdatedAtUTC = &t
	}
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
