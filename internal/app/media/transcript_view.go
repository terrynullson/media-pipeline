package mediaapp

import (
	"context"
	"fmt"
	"strings"

	"media-pipeline/internal/domain/job"
	domainmedia "media-pipeline/internal/domain/media"
	domainsummary "media-pipeline/internal/domain/summary"
	"media-pipeline/internal/domain/transcript"
	"media-pipeline/internal/domain/transcription"
	domaintrigger "media-pipeline/internal/domain/trigger"
)

type TranscriptMediaReader interface {
	GetByID(ctx context.Context, id int64) (domainmedia.Media, error)
}

type TranscriptReader interface {
	GetByMediaID(ctx context.Context, mediaID int64) (transcript.Transcript, bool, error)
}

type TranscriptJobReader interface {
	FindLatestByMediaAndType(ctx context.Context, mediaID int64, jobType job.Type) (job.Job, bool, error)
	ListByMediaID(ctx context.Context, mediaID int64) ([]job.Job, error)
}

type TriggerEventReader interface {
	ListByMediaID(ctx context.Context, mediaID int64) ([]domaintrigger.Event, error)
}

type TriggerScreenshotReader interface {
	ListByMediaID(ctx context.Context, mediaID int64) ([]domaintrigger.Screenshot, error)
}

type SummaryReader interface {
	GetByMediaID(ctx context.Context, mediaID int64) (domainsummary.Summary, bool, error)
}

type TranscriptViewUseCase struct {
	mediaRepo      TranscriptMediaReader
	transcriptRepo TranscriptReader
	triggerEvents  TriggerEventReader
	screenshots    TriggerScreenshotReader
	summaries      SummaryReader
	jobRepo        TranscriptJobReader
}

type TranscriptViewResult struct {
	Media               domainmedia.Media
	Transcript          transcript.Transcript
	HasTranscript       bool
	TriggerEvents       []domaintrigger.Event
	TriggerScreenshots  map[int64]domaintrigger.Screenshot
	ExtractAudioJob     *job.Job
	TranscribeJob       *job.Job
	AnalyzeJob          *job.Job
	ScreenshotJob       *job.Job
	Summary             domainsummary.Summary
	HasSummary          bool
	SummaryJob          *job.Job
	LatestFailedJob     *job.Job
	Settings            *transcription.Settings
	SettingsUnavailable bool
}

func NewTranscriptViewUseCase(
	mediaRepo TranscriptMediaReader,
	transcriptRepo TranscriptReader,
	triggerEventRepo TriggerEventReader,
	triggerScreenshotRepo TriggerScreenshotReader,
	summaryRepo SummaryReader,
	jobRepo TranscriptJobReader,
) *TranscriptViewUseCase {
	return &TranscriptViewUseCase{
		mediaRepo:      mediaRepo,
		transcriptRepo: transcriptRepo,
		triggerEvents:  triggerEventRepo,
		screenshots:    triggerScreenshotRepo,
		summaries:      summaryRepo,
		jobRepo:        jobRepo,
	}
}

func (u *TranscriptViewUseCase) Load(ctx context.Context, mediaID int64) (TranscriptViewResult, error) {
	mediaItem, err := u.mediaRepo.GetByID(ctx, mediaID)
	if err != nil {
		return TranscriptViewResult{}, fmt.Errorf("load media for transcript view: %w", err)
	}

	result := TranscriptViewResult{
		Media: mediaItem,
	}

	transcriptItem, ok, err := u.transcriptRepo.GetByMediaID(ctx, mediaID)
	if err != nil {
		return TranscriptViewResult{}, fmt.Errorf("load transcript for media %d: %w", mediaID, err)
	}
	if ok {
		result.Transcript = transcriptItem
		result.HasTranscript = true
	}

	triggerEvents, err := u.triggerEvents.ListByMediaID(ctx, mediaID)
	if err != nil {
		return TranscriptViewResult{}, fmt.Errorf("load trigger events for media %d: %w", mediaID, err)
	}
	result.TriggerEvents = triggerEvents

	screenshots, err := u.screenshots.ListByMediaID(ctx, mediaID)
	if err != nil {
		return TranscriptViewResult{}, fmt.Errorf("load trigger screenshots for media %d: %w", mediaID, err)
	}
	result.TriggerScreenshots = make(map[int64]domaintrigger.Screenshot, len(screenshots))
	for _, item := range screenshots {
		result.TriggerScreenshots[item.TriggerEventID] = item
	}

	jobs, err := u.jobRepo.ListByMediaID(ctx, mediaID)
	if err != nil {
		return TranscriptViewResult{}, fmt.Errorf("load jobs for media %d: %w", mediaID, err)
	}
	jobsByType := latestJobsByType(jobs)
	result.LatestFailedJob = latestFailedJob(jobs)

	if currentJob, ok := jobsByType[job.TypeExtractAudio]; ok {
		result.ExtractAudioJob = &currentJob
	}
	if currentJob, ok := jobsByType[job.TypeTranscribe]; ok {
		result.TranscribeJob = &currentJob
	}
	if currentJob, ok := jobsByType[job.TypeAnalyzeTriggers]; ok {
		result.AnalyzeJob = &currentJob
	}
	if currentJob, ok := jobsByType[job.TypeExtractScreenshots]; ok {
		result.ScreenshotJob = &currentJob
	}
	if currentJob, ok := jobsByType[job.TypeGenerateSummary]; ok {
		result.SummaryJob = &currentJob
	}

	summaryItem, ok, err := u.summaries.GetByMediaID(ctx, mediaID)
	if err != nil {
		return TranscriptViewResult{}, fmt.Errorf("load summary for media %d: %w", mediaID, err)
	}
	if ok {
		result.Summary = summaryItem
		result.HasSummary = true
	}

	if result.TranscribeJob == nil || strings.TrimSpace(result.TranscribeJob.Payload) == "" {
		return result, nil
	}

	payload, err := job.DecodeTranscribePayload(result.TranscribeJob.Payload)
	if err != nil {
		result.SettingsUnavailable = true
		return result, nil
	}

	settings := transcription.NormalizeSettings(payload.Settings)
	if err := transcription.ValidateSettings(settings); err != nil {
		result.SettingsUnavailable = true
		return result, nil
	}
	result.Settings = &settings

	return result, nil
}

func latestJobsByType(items []job.Job) map[job.Type]job.Job {
	result := make(map[job.Type]job.Job, len(items))
	for _, item := range items {
		if _, ok := result[item.Type]; ok {
			continue
		}
		result[item.Type] = item
	}

	return result
}

func latestFailedJob(items []job.Job) *job.Job {
	for _, item := range items {
		if item.Status != job.StatusFailed {
			continue
		}

		current := item
		return &current
	}

	return nil
}
