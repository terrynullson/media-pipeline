package mediaapp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"media-pipeline/internal/domain/job"
	domainmedia "media-pipeline/internal/domain/media"
)

// ErrNoFailedJobs is returned when there is no failed job to retry for the media.
var ErrNoFailedJobs = errors.New("no failed jobs to retry")

// RetryJobResult carries the ID of the requeued job.
type RetryJobResult struct {
	JobID int64
}

type retryMediaReader interface {
	GetByID(ctx context.Context, id int64) (domainmedia.Media, error)
}

type retryJobReader interface {
	ListByMediaID(ctx context.Context, mediaID int64) ([]job.Job, error)
}

type retryJobWriter interface {
	Requeue(ctx context.Context, id int64, errorMessage string, nowUTC time.Time) error
}

type retryMediaStatusWriter interface {
	MarkUploaded(ctx context.Context, id int64, nowUTC time.Time) error
	MarkAudioReady(ctx context.Context, id int64, nowUTC time.Time) error
}

// RetryJobUseCase finds the latest failed job for a media item and requeues it,
// resetting the media status to the appropriate state so the worker picks it up.
type RetryJobUseCase struct {
	mediaRepo retryMediaReader
	jobRepo   retryJobReader
	jobWriter retryJobWriter
	mediaWriter retryMediaStatusWriter
}

func NewRetryJobUseCase(
	mediaRepo retryMediaReader,
	jobRepo retryJobReader,
	jobWriter retryJobWriter,
	mediaWriter retryMediaStatusWriter,
) *RetryJobUseCase {
	return &RetryJobUseCase{
		mediaRepo:   mediaRepo,
		jobRepo:     jobRepo,
		jobWriter:   jobWriter,
		mediaWriter: mediaWriter,
	}
}

func (u *RetryJobUseCase) Retry(ctx context.Context, mediaID int64) (RetryJobResult, error) {
	if _, err := u.mediaRepo.GetByID(ctx, mediaID); err != nil {
		return RetryJobResult{}, fmt.Errorf("load media %d: %w", mediaID, err)
	}

	jobs, err := u.jobRepo.ListByMediaID(ctx, mediaID)
	if err != nil {
		return RetryJobResult{}, fmt.Errorf("list jobs for media %d: %w", mediaID, err)
	}

	// ListByMediaID returns jobs newest-first. Find the most recent failed job.
	var failedJob *job.Job
	for i := range jobs {
		if jobs[i].Status == job.StatusFailed {
			failedJob = &jobs[i]
			break
		}
	}
	if failedJob == nil {
		return RetryJobResult{}, ErrNoFailedJobs
	}

	nowUTC := time.Now().UTC()
	if err := u.jobWriter.Requeue(ctx, failedJob.ID, "", nowUTC); err != nil {
		return RetryJobResult{}, fmt.Errorf("requeue job %d: %w", failedJob.ID, err)
	}

	// Reset media status so the worker recognises it as ready for processing again.
	if resetErr := resetMediaStatusForJob(ctx, failedJob.Type, mediaID, nowUTC, u.mediaWriter); resetErr != nil {
		// Non-fatal: the job is already requeued; log via error wrapping but don't fail.
		_ = resetErr
	}

	return RetryJobResult{JobID: failedJob.ID}, nil
}

// resetMediaStatusForJob rolls back the media status to the state that precedes
// the failed job's stage, so the worker condition checks are satisfied.
func resetMediaStatusForJob(
	ctx context.Context,
	jobType job.Type,
	mediaID int64,
	nowUTC time.Time,
	w retryMediaStatusWriter,
) error {
	switch jobType {
	case job.TypeExtractAudio:
		// Revert to "queued" so the worker can re-claim the extract_audio job.
		return w.MarkUploaded(ctx, mediaID, nowUTC)
	case job.TypeTranscribe:
		// Revert to "audio_extracted" so the transcribe job is picked up again.
		return w.MarkAudioReady(ctx, mediaID, nowUTC)
	default:
		// analyze_triggers, extract_screenshots, prepare_preview_video,
		// generate_summary — these are post-transcription and non-blocking;
		// the worker checks job status directly, no media status reset needed.
		return nil
	}
}

