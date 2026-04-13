package mediaapp

import (
	"context"
	"fmt"

	"media-pipeline/internal/domain/job"
	domainmedia "media-pipeline/internal/domain/media"
)

// MediaStatusResult holds the minimal data required by the machine polling
// /status endpoint. It intentionally excludes transcript text, screenshots,
// trigger events, and summaries — those are only needed by /result.
type MediaStatusResult struct {
	Media           domainmedia.Media
	ExtractAudioJob *job.Job
	TranscribeJob   *job.Job
	PreviewJob      *job.Job
	AnalyzeJob      *job.Job
	ScreenshotJob   *job.Job
	HasTranscript   bool
}

// MediaStatusMediaReader is the media read interface required by MediaStatusUseCase.
type MediaStatusMediaReader interface {
	GetByID(ctx context.Context, id int64) (domainmedia.Media, error)
}

// MediaStatusTranscriptChecker checks whether a transcript row exists for a
// media item without loading the full transcript text.
type MediaStatusTranscriptChecker interface {
	ExistsByMediaID(ctx context.Context, mediaID int64) (bool, error)
}

// MediaStatusJobReader lists all jobs for a media item.
type MediaStatusJobReader interface {
	ListByMediaID(ctx context.Context, mediaID int64) ([]job.Job, error)
}

// MediaStatusUseCase loads only the data required for the /status endpoint:
// media row + jobs + transcript existence flag.
type MediaStatusUseCase struct {
	mediaRepo      MediaStatusMediaReader
	transcriptRepo MediaStatusTranscriptChecker
	jobRepo        MediaStatusJobReader
}

func NewMediaStatusUseCase(
	mediaRepo MediaStatusMediaReader,
	transcriptRepo MediaStatusTranscriptChecker,
	jobRepo MediaStatusJobReader,
) *MediaStatusUseCase {
	return &MediaStatusUseCase{
		mediaRepo:      mediaRepo,
		transcriptRepo: transcriptRepo,
		jobRepo:        jobRepo,
	}
}

func (u *MediaStatusUseCase) Load(ctx context.Context, mediaID int64) (MediaStatusResult, error) {
	mediaItem, err := u.mediaRepo.GetByID(ctx, mediaID)
	if err != nil {
		return MediaStatusResult{}, fmt.Errorf("load media for status: %w", err)
	}

	jobs, err := u.jobRepo.ListByMediaID(ctx, mediaID)
	if err != nil {
		return MediaStatusResult{}, fmt.Errorf("load jobs for media %d: %w", mediaID, err)
	}

	hasTranscript, err := u.transcriptRepo.ExistsByMediaID(ctx, mediaID)
	if err != nil {
		return MediaStatusResult{}, fmt.Errorf("check transcript for media %d: %w", mediaID, err)
	}

	result := MediaStatusResult{
		Media:         mediaItem,
		HasTranscript: hasTranscript,
	}

	jobsByType := latestJobByType(jobs)
	if j, ok := jobsByType[job.TypeExtractAudio]; ok {
		result.ExtractAudioJob = &j
	}
	if j, ok := jobsByType[job.TypeTranscribe]; ok {
		result.TranscribeJob = &j
	}
	if j, ok := jobsByType[job.TypePreparePreviewVideo]; ok {
		result.PreviewJob = &j
	}
	if j, ok := jobsByType[job.TypeAnalyzeTriggers]; ok {
		result.AnalyzeJob = &j
	}
	if j, ok := jobsByType[job.TypeExtractScreenshots]; ok {
		result.ScreenshotJob = &j
	}

	return result, nil
}
