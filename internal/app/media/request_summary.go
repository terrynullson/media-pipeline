package mediaapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"media-pipeline/internal/domain/job"
	domainmedia "media-pipeline/internal/domain/media"
	"media-pipeline/internal/domain/transcript"
)

var ErrSummaryTranscriptNotReady = errors.New("summary transcript is not ready")

type SummaryRequestMediaReader interface {
	GetByID(ctx context.Context, id int64) (domainmedia.Media, error)
}

type SummaryRequestTranscriptReader interface {
	GetByMediaID(ctx context.Context, mediaID int64) (transcript.Transcript, bool, error)
}

type SummaryRequestJobRepository interface {
	Create(ctx context.Context, j job.Job) (int64, error)
	FindLatestByMediaAndType(ctx context.Context, mediaID int64, jobType job.Type) (job.Job, bool, error)
}

type RequestSummaryResult struct {
	MediaID         int64
	Created         bool
	AlreadyInFlight bool
}

type RequestSummaryUseCase struct {
	mediaRepo      SummaryRequestMediaReader
	transcriptRepo SummaryRequestTranscriptReader
	jobRepo        SummaryRequestJobRepository
}

func NewRequestSummaryUseCase(
	mediaRepo SummaryRequestMediaReader,
	transcriptRepo SummaryRequestTranscriptReader,
	jobRepo SummaryRequestJobRepository,
) *RequestSummaryUseCase {
	return &RequestSummaryUseCase{
		mediaRepo:      mediaRepo,
		transcriptRepo: transcriptRepo,
		jobRepo:        jobRepo,
	}
}

func (u *RequestSummaryUseCase) Request(ctx context.Context, mediaID int64) (RequestSummaryResult, error) {
	if _, err := u.mediaRepo.GetByID(ctx, mediaID); err != nil {
		return RequestSummaryResult{}, fmt.Errorf("load media for summary request: %w", err)
	}

	transcriptItem, ok, err := u.transcriptRepo.GetByMediaID(ctx, mediaID)
	if err != nil {
		return RequestSummaryResult{}, fmt.Errorf("load transcript for summary request: %w", err)
	}
	if !ok || strings.TrimSpace(transcriptItem.FullText) == "" {
		return RequestSummaryResult{}, ErrSummaryTranscriptNotReady
	}

	currentJob, ok, err := u.jobRepo.FindLatestByMediaAndType(ctx, mediaID, job.TypeGenerateSummary)
	if err != nil {
		return RequestSummaryResult{}, fmt.Errorf("load latest summary job: %w", err)
	}
	if ok && (currentJob.Status == job.StatusPending || currentJob.Status == job.StatusRunning) {
		return RequestSummaryResult{
			MediaID:         mediaID,
			AlreadyInFlight: true,
		}, nil
	}

	nowUTC := time.Now().UTC()
	if _, err := u.jobRepo.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeGenerateSummary,
		Status:       job.StatusPending,
		Attempts:     0,
		ErrorMessage: "",
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	}); err != nil {
		return RequestSummaryResult{}, fmt.Errorf("create summary job: %w", err)
	}

	return RequestSummaryResult{
		MediaID: mediaID,
		Created: true,
	}, nil
}
