package mediaapp

import (
	"context"
	"fmt"
	"strings"

	"media-pipeline/internal/domain/job"
	domainmedia "media-pipeline/internal/domain/media"
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
}

type TriggerEventReader interface {
	ListByMediaID(ctx context.Context, mediaID int64) ([]domaintrigger.Event, error)
}

type TranscriptViewUseCase struct {
	mediaRepo      TranscriptMediaReader
	transcriptRepo TranscriptReader
	triggerEvents  TriggerEventReader
	jobRepo        TranscriptJobReader
}

type TranscriptViewResult struct {
	Media               domainmedia.Media
	Transcript          transcript.Transcript
	HasTranscript       bool
	TriggerEvents       []domaintrigger.Event
	AnalyzeJob          *job.Job
	Settings            *transcription.Settings
	SettingsUnavailable bool
}

func NewTranscriptViewUseCase(
	mediaRepo TranscriptMediaReader,
	transcriptRepo TranscriptReader,
	triggerEventRepo TriggerEventReader,
	jobRepo TranscriptJobReader,
) *TranscriptViewUseCase {
	return &TranscriptViewUseCase{
		mediaRepo:      mediaRepo,
		transcriptRepo: transcriptRepo,
		triggerEvents:  triggerEventRepo,
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

	analyzeJob, ok, err := u.jobRepo.FindLatestByMediaAndType(ctx, mediaID, job.TypeAnalyzeTriggers)
	if err != nil {
		return TranscriptViewResult{}, fmt.Errorf("load analyze job for media %d: %w", mediaID, err)
	}
	if ok {
		result.AnalyzeJob = &analyzeJob
	}

	currentJob, ok, err := u.jobRepo.FindLatestByMediaAndType(ctx, mediaID, job.TypeTranscribe)
	if err != nil {
		return TranscriptViewResult{}, fmt.Errorf("load transcription job for media %d: %w", mediaID, err)
	}
	if !ok || strings.TrimSpace(currentJob.Payload) == "" {
		return result, nil
	}

	payload, err := job.DecodeTranscribePayload(currentJob.Payload)
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
