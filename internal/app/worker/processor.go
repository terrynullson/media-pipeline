package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/media"
	"media-pipeline/internal/domain/ports"
	domainsummary "media-pipeline/internal/domain/summary"
	"media-pipeline/internal/domain/transcript"
	"media-pipeline/internal/domain/transcription"
	domaintrigger "media-pipeline/internal/domain/trigger"
)

type JobRepository interface {
	Create(ctx context.Context, j job.Job) (int64, error)
	ExistsActiveOrDone(ctx context.Context, mediaID int64, jobType job.Type) (bool, error)
	ClaimNextPending(ctx context.Context, jobType job.Type, nowUTC time.Time) (job.Job, bool, error)
	MarkDone(ctx context.Context, id int64, nowUTC time.Time) error
	MarkFailed(ctx context.Context, id int64, errorMessage string, nowUTC time.Time) error
	UpdateProgress(ctx context.Context, id int64, progressPercent *float64, progressLabel string, isEstimate bool, nowUTC time.Time) error
	ListByStatus(ctx context.Context, jobType job.Type, status job.Status) ([]job.Job, error)
	Requeue(ctx context.Context, id int64, errorMessage string, nowUTC time.Time) error
}

type MediaRepository interface {
	GetByID(ctx context.Context, id int64) (media.Media, error)
	MarkProcessing(ctx context.Context, id int64, nowUTC time.Time) error
	MarkAudioExtracted(ctx context.Context, id int64, extractedAudioPath string, nowUTC time.Time) error
	MarkAudioReady(ctx context.Context, id int64, nowUTC time.Time) error
	MarkTranscribing(ctx context.Context, id int64, nowUTC time.Time) error
	MarkTranscribed(ctx context.Context, id int64, transcriptText string, nowUTC time.Time) error
	MarkFailed(ctx context.Context, id int64, nowUTC time.Time) error
	MarkUploaded(ctx context.Context, id int64, nowUTC time.Time) error
}

type TranscriptRepository interface {
	Save(ctx context.Context, item transcript.Transcript) error
	GetByMediaID(ctx context.Context, mediaID int64) (transcript.Transcript, bool, error)
}

type TriggerRuleRepository interface {
	ListEnabled(ctx context.Context) ([]domaintrigger.Rule, error)
}

type TriggerEventRepository interface {
	ReplaceForMedia(ctx context.Context, mediaID int64, transcriptID *int64, events []domaintrigger.Event) error
	ListByMediaID(ctx context.Context, mediaID int64) ([]domaintrigger.Event, error)
}

type TriggerScreenshotRepository interface {
	ReplaceForMedia(ctx context.Context, mediaID int64, items []domaintrigger.Screenshot) error
	ListByMediaID(ctx context.Context, mediaID int64) ([]domaintrigger.Screenshot, error)
	ListPathsByMediaID(ctx context.Context, mediaID int64) ([]string, error)
}

type TranscriptionProfileProvider interface {
	GetCurrent(ctx context.Context) (transcription.Profile, error)
}

type SummaryRepository interface {
	Save(ctx context.Context, item domainsummary.Summary) error
}

type Processor struct {
	jobs                  JobRepository
	media                 MediaRepository
	transcripts           TranscriptRepository
	triggerRules          TriggerRuleRepository
	triggerEvents         TriggerEventRepository
	triggerScreenshots    TriggerScreenshotRepository
	summaries             SummaryRepository
	audioExtractor        ports.AudioExtractor
	audioDurations        ports.AudioDurationReader
	screenshotExtractor   ports.ScreenshotExtractor
	transcriber           ports.Transcriber
	summarizer            ports.Summarizer
	profiles              TranscriptionProfileProvider
	logger                *slog.Logger
	uploadDir             string
	audioDir              string
	screenshotsDir        string
	ffmpegTimeout         time.Duration
	screenshotTimeout     time.Duration
	transcribeBaseTimeout time.Duration
}

func NewProcessor(
	jobRepo JobRepository,
	mediaRepo MediaRepository,
	transcriptRepo TranscriptRepository,
	triggerRuleRepo TriggerRuleRepository,
	triggerEventRepo TriggerEventRepository,
	triggerScreenshotRepo TriggerScreenshotRepository,
	summaryRepo SummaryRepository,
	audioExtractor ports.AudioExtractor,
	audioDurationReader ports.AudioDurationReader,
	screenshotExtractor ports.ScreenshotExtractor,
	transcriber ports.Transcriber,
	summarizer ports.Summarizer,
	profiles TranscriptionProfileProvider,
	uploadDir string,
	audioDir string,
	screenshotsDir string,
	ffmpegTimeout time.Duration,
	screenshotTimeout time.Duration,
	transcribeTimeout time.Duration,
	logger *slog.Logger,
) *Processor {
	return &Processor{
		jobs:                  jobRepo,
		media:                 mediaRepo,
		transcripts:           transcriptRepo,
		triggerRules:          triggerRuleRepo,
		triggerEvents:         triggerEventRepo,
		triggerScreenshots:    triggerScreenshotRepo,
		summaries:             summaryRepo,
		audioExtractor:        audioExtractor,
		audioDurations:        audioDurationReader,
		screenshotExtractor:   screenshotExtractor,
		transcriber:           transcriber,
		summarizer:            summarizer,
		profiles:              profiles,
		logger:                logger,
		uploadDir:             uploadDir,
		audioDir:              audioDir,
		screenshotsDir:        screenshotsDir,
		ffmpegTimeout:         ffmpegTimeout,
		screenshotTimeout:     screenshotTimeout,
		transcribeBaseTimeout: transcribeTimeout,
	}
}

func (p *Processor) ProcessNext(ctx context.Context) (bool, error) {
	for _, jobType := range []job.Type{
		job.TypeExtractAudio,
		job.TypeTranscribe,
		job.TypeAnalyzeTriggers,
		job.TypeExtractScreenshots,
		job.TypeGenerateSummary,
	} {
		processed, err := p.processNextByType(ctx, jobType)
		if err != nil {
			return false, err
		}
		if processed {
			return true, nil
		}
	}

	return false, nil
}

func (p *Processor) processNextByType(ctx context.Context, jobType job.Type) (bool, error) {
	nowUTC := time.Now().UTC()
	claimedJob, ok, err := p.jobs.ClaimNextPending(ctx, jobType, nowUTC)
	if err != nil {
		return false, fmt.Errorf("claim next pending job: %w", err)
	}
	if !ok {
		return false, nil
	}

	logger := p.logger.With(
		slog.Int64("job_id", claimedJob.ID),
		slog.Int64("media_id", claimedJob.MediaID),
		slog.String("job_type", string(claimedJob.Type)),
	)
	logger.Info("job claimed")

	switch claimedJob.Type {
	case job.TypeExtractAudio:
		p.processExtractAudioJob(ctx, claimedJob, logger)
	case job.TypeTranscribe:
		p.processTranscribeJob(ctx, claimedJob, logger)
	case job.TypeAnalyzeTriggers:
		p.processAnalyzeTriggersJob(ctx, claimedJob, logger)
	case job.TypeExtractScreenshots:
		p.processExtractScreenshotsJob(ctx, claimedJob, logger)
	case job.TypeGenerateSummary:
		p.processGenerateSummaryJob(ctx, claimedJob, logger)
	default:
		jobLog := newJobExecutionLog(logger)
		p.failJob(
			ctx,
			claimedJob,
			0,
			fmt.Sprintf("Неподдерживаемый тип задачи: %q", claimedJob.Type),
			true,
			jobLog,
		)
	}

	return true, nil
}

func (p *Processor) processExtractAudioJob(ctx context.Context, claimedJob job.Job, logger *slog.Logger) {
	jobLog := newJobExecutionLog(logger)

	mediaItem, err := p.media.GetByID(ctx, claimedJob.MediaID)
	if err != nil {
		p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось загрузить медиа", err), true, jobLog, slog.Any("error", err))
		return
	}

	if err := p.media.MarkProcessing(ctx, mediaItem.ID, time.Now().UTC()); err != nil {
		p.failJob(ctx, claimedJob, mediaItem.ID, buildInternalFailureMessage("Не удалось отметить медиа как обрабатываемое", err), true, jobLog, slog.Any("error", err))
		return
	}

	inputPath, err := safeJoinBasePath(p.uploadDir, mediaItem.StoragePath)
	if err != nil {
		p.failJob(ctx, claimedJob, mediaItem.ID, buildInternalFailureMessage("Не удалось подготовить путь к исходному файлу", err), true, jobLog, slog.Any("error", err))
		return
	}

	ffmpegCtx, cancel := context.WithTimeout(ctx, p.ffmpegTimeout)
	defer cancel()

	jobLog.logger.Info("starting ffmpeg",
		slog.String("input_path", inputPath),
		slog.String("audio_dir", p.audioDir),
		slog.Duration("timeout", p.ffmpegTimeout),
	)

	extractResult, err := p.audioExtractor.Extract(ffmpegCtx, ports.ExtractAudioInput{
		MediaID:     mediaItem.ID,
		InputPath:   inputPath,
		StoredName:  mediaItem.StoredName,
		OutputDir:   p.audioDir,
		ProcessedAt: time.Now().UTC().Format("2006-01-02"),
	})
	if err != nil {
		diagnostic := compactDiagnostic(extractResult.Stderr, 500)
		p.failJob(
			ctx,
			claimedJob,
			mediaItem.ID,
			buildUserFacingStageError(job.TypeExtractAudio, err, extractResult.Stderr),
			true,
			jobLog,
			slog.Any("error", err),
			slog.String("stderr_excerpt", diagnostic),
		)
		return
	}
	jobLog.logger.Info("ffmpeg completed",
		slog.String("audio_path", extractResult.OutputPath),
		slog.String("stderr_excerpt", compactDiagnostic(extractResult.Stderr, 500)),
	)

	if err := p.media.MarkAudioExtracted(ctx, mediaItem.ID, extractResult.OutputPath, time.Now().UTC()); err != nil {
		_ = cleanupOutputFile(p.audioDir, extractResult.OutputPath)
		p.failJob(ctx, claimedJob, mediaItem.ID, buildInternalFailureMessage("Не удалось сохранить путь к извлечённому аудио", err), true, jobLog, slog.Any("error", err))
		return
	}

	if err := p.enqueueTranscribeJob(ctx, mediaItem.ID); err != nil {
		p.failJob(ctx, claimedJob, mediaItem.ID, buildInternalFailureMessage("Не удалось поставить распознавание в очередь", err), true, jobLog, slog.Any("error", err))
		return
	}

	if err := p.jobs.MarkDone(ctx, claimedJob.ID, time.Now().UTC()); err != nil {
		_ = cleanupOutputFile(p.audioDir, extractResult.OutputPath)
		p.failJob(ctx, claimedJob, mediaItem.ID, buildInternalFailureMessage("Не удалось завершить задачу извлечения аудио", err), true, jobLog, slog.Any("error", err))
		return
	}

	jobLog.Success(
		slog.String("audio_path", extractResult.OutputPath),
	)
}

func (p *Processor) processTranscribeJob(ctx context.Context, claimedJob job.Job, logger *slog.Logger) {
	jobLog := newJobExecutionLog(logger)

	mediaItem, err := p.media.GetByID(ctx, claimedJob.MediaID)
	if err != nil {
		p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось загрузить медиа", err), true, jobLog, slog.Any("error", err))
		return
	}

	if strings.TrimSpace(mediaItem.ExtractedAudioPath) == "" {
		p.failJob(ctx, claimedJob, mediaItem.ID, "У файла не найден путь к извлечённому аудио.", true, jobLog)
		return
	}

	if err := p.media.MarkTranscribing(ctx, mediaItem.ID, time.Now().UTC()); err != nil {
		p.failJob(ctx, claimedJob, mediaItem.ID, buildInternalFailureMessage("Не удалось отметить медиа как распознаваемое", err), true, jobLog, slog.Any("error", err))
		return
	}

	audioPath, err := safeJoinBasePath(p.audioDir, mediaItem.ExtractedAudioPath)
	if err != nil {
		p.failJob(ctx, claimedJob, mediaItem.ID, buildInternalFailureMessage("Не удалось подготовить путь к аудио", err), true, jobLog, slog.Any("error", err))
		return
	}

	settings, err := p.resolveTranscriptionSettings(ctx, claimedJob, logger)
	if err != nil {
		p.failJob(ctx, claimedJob, mediaItem.ID, buildInternalFailureMessage("Не удалось прочитать настройки распознавания", err), true, jobLog, slog.Any("error", err))
		return
	}

	if p.audioDurations == nil {
		p.failJob(ctx, claimedJob, mediaItem.ID, "Не удалось определить длительность файла перед распознаванием текста.", true, jobLog)
		return
	}

	audioDuration, err := p.audioDurations.ReadDuration(audioPath)
	if err != nil {
		p.failJob(
			ctx,
			claimedJob,
			mediaItem.ID,
			"Не удалось определить длительность файла перед распознаванием текста.",
			true,
			jobLog,
			slog.Any("error", err),
			slog.String("audio_path", audioPath),
		)
		return
	}

	policy := transcription.EvaluateRuntimePolicy(settings, audioDuration, p.transcribeBaseTimeout)
	jobLog.logger.Info("transcription policy decided", attrsToAnySlice(transcriptionPolicyLogAttrs(policy))...)

	if policy.Blocked {
		policyAttrs := transcriptionPolicyLogAttrs(policy)
		p.failJob(
			ctx,
			claimedJob,
			mediaItem.ID,
			buildTranscriptionBlockedFailure(policy),
			true,
			jobLog,
			policyAttrs...,
		)
		return
	}

	transcribeCtx, cancel := context.WithTimeout(ctx, policy.EffectiveTimeout)
	defer cancel()

	jobLog.logger.Info("starting transcription",
		slog.String("audio_path", audioPath),
		slog.Duration("audio_duration", audioDuration),
		slog.String("duration_class", string(policy.DurationClass)),
		slog.String("backend", string(settings.Backend)),
		slog.String("model_name", settings.ModelName),
		slog.String("device", settings.Device),
		slog.String("compute_type", settings.ComputeType),
		slog.String("language", settings.Language),
		slog.Int("beam_size", settings.BeamSize),
		slog.Bool("vad_enabled", settings.VADEnabled),
		slog.Duration("base_timeout", policy.BaseTimeout),
		slog.Duration("timeout", policy.EffectiveTimeout),
	)

	result, err := p.transcriber.Transcribe(transcribeCtx, ports.TranscribeInput{
		AudioPath: audioPath,
		Settings:  settings,
		Progress: func(progress ports.TranscriptionProgress) {
			progressValue := progress.Percent
			progressLabel := "Оценка по обработанным сегментам"
			progressCtx, cancelProgress := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancelProgress()

			if updateErr := p.jobs.UpdateProgress(
				progressCtx,
				claimedJob.ID,
				&progressValue,
				progressLabel,
				progress.IsEstimate,
				time.Now().UTC(),
			); updateErr != nil {
				jobLog.logger.Warn("persist transcription progress failed", slog.Any("error", updateErr))
			}
		},
	})
	if err != nil {
		diagnostics := transcriptionDiagnostics(err)
		failureMessage := buildUserFacingStageError(job.TypeTranscribe, err, diagnostics)
		if errors.Is(err, context.DeadlineExceeded) {
			failureMessage = buildTranscriptionTimeoutFailure(settings, policy)
		}
		p.failJob(
			ctx,
			claimedJob,
			mediaItem.ID,
			failureMessage,
			true,
			jobLog,
			slog.Any("error", err),
			slog.String("diagnostics_excerpt", compactDiagnostic(diagnostics, 500)),
			slog.String("audio_path", audioPath),
			slog.Duration("audio_duration", audioDuration),
			slog.String("duration_class", string(policy.DurationClass)),
			slog.Duration("effective_timeout", policy.EffectiveTimeout),
		)
		return
	}

	jobLog.logger.Info("transcription completed",
		slog.Int("segments", len(result.Segments)),
	)

	nowUTC := time.Now().UTC()
	if err := p.transcripts.Save(ctx, transcript.Transcript{
		MediaID:      mediaItem.ID,
		Language:     settings.Language,
		FullText:     result.FullText,
		Segments:     toTranscriptSegments(result.Segments),
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	}); err != nil {
		p.failJob(ctx, claimedJob, mediaItem.ID, buildInternalFailureMessage("Не удалось сохранить расшифровку", err), true, jobLog, slog.Any("error", err))
		return
	}
	jobLog.logger.Info("transcript persisted successfully", slog.Int64("media_id", mediaItem.ID))

	if err := p.media.MarkTranscribed(ctx, mediaItem.ID, result.FullText, time.Now().UTC()); err != nil {
		p.failJob(ctx, claimedJob, mediaItem.ID, buildInternalFailureMessage("Не удалось отметить медиа как распознанное", err), true, jobLog, slog.Any("error", err))
		return
	}

	if err := p.enqueueNextJob(ctx, mediaItem.ID, job.TypeAnalyzeTriggers); err != nil {
		p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось поставить анализ триггеров в очередь", err), false, jobLog, slog.Any("error", err))
		return
	}

	if err := p.jobs.MarkDone(ctx, claimedJob.ID, time.Now().UTC()); err != nil {
		p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось завершить задачу распознавания", err), false, jobLog, slog.Any("error", err))
		return
	}

	jobLog.Success(slog.Int("segments", len(result.Segments)))
}

func (p *Processor) processAnalyzeTriggersJob(ctx context.Context, claimedJob job.Job, logger *slog.Logger) {
	jobLog := newJobExecutionLog(logger)

	transcriptItem, ok, err := p.transcripts.GetByMediaID(ctx, claimedJob.MediaID)
	if err != nil {
		p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось загрузить расшифровку для анализа триггеров", err), false, jobLog, slog.Any("error", err))
		return
	}
	if !ok {
		p.failJob(ctx, claimedJob, 0, "Расшифровка не найдена, поэтому анализ триггеров не может продолжиться.", false, jobLog)
		return
	}

	rules, err := p.triggerRules.ListEnabled(ctx)
	if err != nil {
		p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось загрузить включённые правила триггеров", err), false, jobLog, slog.Any("error", err))
		return
	}

	nowUTC := time.Now().UTC()
	transcriptID := transcriptItem.ID
	events := domaintrigger.DetectEvents(domaintrigger.MatchInput{
		MediaID:      claimedJob.MediaID,
		TranscriptID: &transcriptID,
		Segments:     transcriptItem.Segments,
		Rules:        rules,
		CreatedAtUTC: nowUTC,
	})

	if err := p.triggerEvents.ReplaceForMedia(ctx, claimedJob.MediaID, &transcriptID, events); err != nil {
		p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось сохранить найденные триггеры", err), false, jobLog, slog.Any("error", err))
		return
	}

	if err := p.enqueueNextJob(ctx, claimedJob.MediaID, job.TypeExtractScreenshots); err != nil {
		p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось поставить подготовку скриншотов в очередь", err), false, jobLog, slog.Any("error", err))
		return
	}

	if err := p.jobs.MarkDone(ctx, claimedJob.ID, nowUTC); err != nil {
		p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось завершить задачу анализа триггеров", err), false, jobLog, slog.Any("error", err))
		return
	}

	jobLog.Success(slog.Int("events", len(events)))
}

func (p *Processor) processExtractScreenshotsJob(ctx context.Context, claimedJob job.Job, logger *slog.Logger) {
	jobLog := newJobExecutionLog(logger)

	mediaItem, err := p.media.GetByID(ctx, claimedJob.MediaID)
	if err != nil {
		p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось загрузить медиа для скриншотов", err), false, jobLog, slog.Any("error", err))
		return
	}

	if mediaItem.IsAudioOnly() {
		existingPaths, err := p.triggerScreenshots.ListPathsByMediaID(ctx, mediaItem.ID)
		if err != nil {
			p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось загрузить текущие скриншоты", err), false, jobLog, slog.Any("error", err))
			return
		}
		if err := p.triggerScreenshots.ReplaceForMedia(ctx, mediaItem.ID, nil); err != nil {
			p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось очистить скриншоты для аудио", err), false, jobLog, slog.Any("error", err))
			return
		}
		p.cleanupCreatedScreenshots(existingPaths, logger)
		if err := p.jobs.MarkDone(ctx, claimedJob.ID, time.Now().UTC()); err != nil {
			p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось завершить задачу скриншотов", err), false, jobLog, slog.Any("error", err))
			return
		}
		jobLog.Success(slog.String("result", "skipped for audio-only media"))
		return
	}

	inputPath, err := safeJoinBasePath(p.uploadDir, mediaItem.StoragePath)
	if err != nil {
		p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось подготовить путь к видео для скриншотов", err), false, jobLog, slog.Any("error", err))
		return
	}

	events, err := p.triggerEvents.ListByMediaID(ctx, claimedJob.MediaID)
	if err != nil {
		p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось загрузить найденные триггеры для скриншотов", err), false, jobLog, slog.Any("error", err))
		return
	}

	if len(events) == 0 {
		existingPaths, err := p.triggerScreenshots.ListPathsByMediaID(ctx, mediaItem.ID)
		if err != nil {
			p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось загрузить текущие скриншоты", err), false, jobLog, slog.Any("error", err))
			return
		}
		if err := p.triggerScreenshots.ReplaceForMedia(ctx, mediaItem.ID, nil); err != nil {
			p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось очистить скриншоты без совпадений", err), false, jobLog, slog.Any("error", err))
			return
		}
		p.cleanupCreatedScreenshots(existingPaths, logger)
		if err := p.jobs.MarkDone(ctx, claimedJob.ID, time.Now().UTC()); err != nil {
			p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось завершить задачу скриншотов", err), false, jobLog, slog.Any("error", err))
			return
		}
		jobLog.Success(slog.String("result", "no trigger events"))
		return
	}

	nowUTC := time.Now().UTC()
	existingPaths, err := p.triggerScreenshots.ListPathsByMediaID(ctx, mediaItem.ID)
	if err != nil {
		p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось загрузить текущие скриншоты", err), false, jobLog, slog.Any("error", err))
		return
	}
	screenshots := make([]domaintrigger.Screenshot, 0, len(events))
	createdPaths := make([]string, 0, len(events))
	for _, event := range events {
		if event.StartSec < 0 {
			p.cleanupCreatedScreenshots(createdPaths, logger)
			p.failJob(ctx, claimedJob, 0, "Невалидная временная метка для создания скриншота.", false, jobLog, slog.Float64("timestamp_sec", event.StartSec), slog.Int64("trigger_event_id", event.ID))
			return
		}

		screenshotCtx, cancel := context.WithTimeout(ctx, p.screenshotTimeout)
		result, extractErr := p.screenshotExtractor.Extract(screenshotCtx, ports.ExtractScreenshotInput{
			MediaID:        mediaItem.ID,
			TriggerEventID: event.ID,
			InputPath:      inputPath,
			TimestampSec:   event.StartSec,
			OutputDir:      p.screenshotsDir,
			ProcessedAt:    nowUTC.Format("2006-01-02"),
		})
		cancel()
		if extractErr != nil {
			p.cleanupCreatedScreenshots(createdPaths, logger)
			p.failJob(
				ctx,
				claimedJob,
				0,
				buildUserFacingStageError(job.TypeExtractScreenshots, extractErr, result.Stderr),
				false,
				jobLog,
				slog.Any("error", extractErr),
				slog.Int64("trigger_event_id", event.ID),
				slog.String("stderr_excerpt", compactDiagnostic(result.Stderr, 500)),
			)
			return
		}

		createdPaths = append(createdPaths, result.ImagePath)
		screenshots = append(screenshots, domaintrigger.Screenshot{
			MediaID:        mediaItem.ID,
			TriggerEventID: event.ID,
			TimestampSec:   event.StartSec,
			ImagePath:      result.ImagePath,
			Width:          result.Width,
			Height:         result.Height,
			CreatedAtUTC:   nowUTC,
		})
	}

	if err := p.triggerScreenshots.ReplaceForMedia(ctx, mediaItem.ID, screenshots); err != nil {
		p.cleanupCreatedScreenshots(createdPaths, logger)
		p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось сохранить скриншоты", err), false, jobLog, slog.Any("error", err))
		return
	}
	p.cleanupCreatedScreenshots(pathsDifference(existingPaths, createdPaths), logger)

	if err := p.jobs.MarkDone(ctx, claimedJob.ID, time.Now().UTC()); err != nil {
		p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось завершить задачу скриншотов", err), false, jobLog, slog.Any("error", err))
		return
	}

	jobLog.Success(slog.Int("screenshots", len(screenshots)))
}

func (p *Processor) processGenerateSummaryJob(ctx context.Context, claimedJob job.Job, logger *slog.Logger) {
	jobLog := newJobExecutionLog(logger)

	transcriptItem, ok, err := p.transcripts.GetByMediaID(ctx, claimedJob.MediaID)
	if err != nil {
		p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось загрузить расшифровку для саммари", err), false, jobLog, slog.Any("error", err))
		return
	}
	if !ok || strings.TrimSpace(transcriptItem.FullText) == "" {
		p.failJob(ctx, claimedJob, 0, "Нельзя собрать саммари без готовой расшифровки.", false, jobLog)
		return
	}

	triggerEvents, err := p.triggerEvents.ListByMediaID(ctx, claimedJob.MediaID)
	if err != nil {
		p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось загрузить триггеры для саммари", err), false, jobLog, slog.Any("error", err))
		return
	}

	triggerScreenshots, err := p.triggerScreenshots.ListByMediaID(ctx, claimedJob.MediaID)
	if err != nil {
		p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось загрузить скриншоты для саммари", err), false, jobLog, slog.Any("error", err))
		return
	}

	jobLog.logger.Info("starting summary generation",
		slog.Int("trigger_events", len(triggerEvents)),
		slog.Int("trigger_screenshots", len(triggerScreenshots)),
	)

	summaryOutput, err := p.summarizer.Generate(ctx, ports.SummaryInput{
		MediaID:            claimedJob.MediaID,
		Transcript:         transcriptItem,
		TriggerEvents:      triggerEvents,
		TriggerScreenshots: triggerScreenshots,
	})
	if err != nil {
		p.failJob(ctx, claimedJob, 0, buildUserFacingStageError(job.TypeGenerateSummary, err, ""), false, jobLog, slog.Any("error", err))
		return
	}

	nowUTC := time.Now().UTC()
	summaryItem := domainsummary.Summary{
		MediaID:      claimedJob.MediaID,
		SummaryText:  summaryOutput.SummaryText,
		Highlights:   append([]string(nil), summaryOutput.Highlights...),
		Provider:     summaryOutput.Provider,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	}
	if err := p.summaries.Save(ctx, summaryItem); err != nil {
		p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось сохранить саммари", err), false, jobLog, slog.Any("error", err))
		return
	}

	if err := p.jobs.MarkDone(ctx, claimedJob.ID, nowUTC); err != nil {
		p.failJob(ctx, claimedJob, 0, buildInternalFailureMessage("Не удалось завершить задачу саммари", err), false, jobLog, slog.Any("error", err))
		return
	}

	jobLog.Success(
		slog.Int("highlights", len(summaryOutput.Highlights)),
		slog.String("provider", summaryOutput.Provider),
	)
}

func (p *Processor) RecoverInterruptedJobs(ctx context.Context) error {
	for _, recovery := range []struct {
		jobType      job.Type
		restoreState func(context.Context, int64, time.Time) error
	}{
		{jobType: job.TypeExtractAudio, restoreState: p.media.MarkUploaded},
		{jobType: job.TypeTranscribe, restoreState: p.media.MarkAudioReady},
		{jobType: job.TypeAnalyzeTriggers},
		{jobType: job.TypeExtractScreenshots},
		{jobType: job.TypeGenerateSummary},
	} {
		if err := p.recoverInterruptedJobType(ctx, recovery.jobType, recovery.restoreState); err != nil {
			return err
		}
	}

	return nil
}

func (p *Processor) failJob(
	ctx context.Context,
	currentJob job.Job,
	mediaID int64,
	failureMessage string,
	markMediaFailed bool,
	jobLog jobExecutionLog,
	logAttrs ...slog.Attr,
) {
	jobLog.Failure(append([]slog.Attr{slog.String("error_message", failureMessage)}, logAttrs...)...)

	if markMediaFailed && mediaID > 0 {
		if err := p.media.MarkFailed(ctx, mediaID, time.Now().UTC()); err != nil {
			jobLog.logger.Error("mark media failed", slog.Any("error", err))
		}
	}

	if err := p.jobs.MarkFailed(ctx, currentJob.ID, truncateMessage(failureMessage, 2000), time.Now().UTC()); err != nil {
		jobLog.logger.Error("mark job failed", slog.Any("error", err))
	}
}

func truncateMessage(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func cleanupOutputFile(audioDir string, relativePath string) error {
	if strings.TrimSpace(relativePath) == "" {
		return nil
	}

	fullPath, err := safeJoinBasePath(audioDir, relativePath)
	if err != nil {
		return err
	}

	if err := os.Remove(fullPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove output file: %w", err)
	}

	return nil
}

func (p *Processor) cleanupCreatedScreenshots(relativePaths []string, logger *slog.Logger) {
	for _, relativePath := range relativePaths {
		if err := cleanupOutputFile(p.screenshotsDir, relativePath); err != nil {
			logger.Error("cleanup screenshot output failed", slog.Any("error", err), slog.String("image_path", relativePath))
		}
	}
}

func pathsDifference(existingPaths []string, keepPaths []string) []string {
	keep := make(map[string]struct{}, len(keepPaths))
	for _, path := range keepPaths {
		keep[path] = struct{}{}
	}

	result := make([]string, 0)
	for _, path := range existingPaths {
		if _, ok := keep[path]; ok {
			continue
		}
		result = append(result, path)
	}

	return result
}

func (p *Processor) enqueueNextJob(ctx context.Context, mediaID int64, jobType job.Type) error {
	exists, err := p.jobs.ExistsActiveOrDone(ctx, mediaID, jobType)
	if err != nil {
		return fmt.Errorf("check existing %s job: %w", jobType, err)
	}
	if exists {
		return nil
	}

	nowUTC := time.Now().UTC()
	if _, err := p.jobs.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         jobType,
		Status:       job.StatusPending,
		Attempts:     0,
		ErrorMessage: "",
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	}); err != nil {
		return fmt.Errorf("create %s job: %w", jobType, err)
	}

	return nil
}

func (p *Processor) enqueueTranscribeJob(ctx context.Context, mediaID int64) error {
	profile, err := p.profiles.GetCurrent(ctx)
	if err != nil {
		return fmt.Errorf("get current transcription profile: %w", err)
	}

	payload, err := job.EncodeTranscribePayload(job.TranscribePayload{
		Settings: transcription.NormalizeSettings(profile.Settings()),
	})
	if err != nil {
		return fmt.Errorf("encode transcribe payload: %w", err)
	}

	exists, err := p.jobs.ExistsActiveOrDone(ctx, mediaID, job.TypeTranscribe)
	if err != nil {
		return fmt.Errorf("check existing %s job: %w", job.TypeTranscribe, err)
	}
	if exists {
		return nil
	}

	nowUTC := time.Now().UTC()
	if _, err := p.jobs.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeTranscribe,
		Payload:      payload,
		Status:       job.StatusPending,
		Attempts:     0,
		ErrorMessage: "",
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	}); err != nil {
		return fmt.Errorf("create %s job: %w", job.TypeTranscribe, err)
	}

	return nil
}

func (p *Processor) resolveTranscriptionSettings(
	ctx context.Context,
	currentJob job.Job,
	logger *slog.Logger,
) (transcription.Settings, error) {
	if strings.TrimSpace(currentJob.Payload) != "" {
		payload, err := job.DecodeTranscribePayload(currentJob.Payload)
		if err != nil {
			return transcription.Settings{}, err
		}
		if err := transcription.ValidateSettings(payload.Settings); err != nil {
			return transcription.Settings{}, err
		}

		return transcription.NormalizeSettings(payload.Settings), nil
	}

	logger.Warn("transcribe job payload is empty, falling back to current transcription profile")
	profile, err := p.profiles.GetCurrent(ctx)
	if err != nil {
		return transcription.Settings{}, fmt.Errorf("get fallback transcription profile: %w", err)
	}

	return transcription.NormalizeSettings(profile.Settings()), nil
}

func (p *Processor) recoverInterruptedJobType(
	ctx context.Context,
	jobType job.Type,
	restoreState func(context.Context, int64, time.Time) error,
) error {
	runningJobs, err := p.jobs.ListByStatus(ctx, jobType, job.StatusRunning)
	if err != nil {
		return fmt.Errorf("list running jobs for %s: %w", jobType, err)
	}
	if len(runningJobs) == 0 {
		return nil
	}

	recoveryMessage := "worker restarted before job completion"
	for _, currentJob := range runningJobs {
		logger := p.logger.With(
			slog.Int64("job_id", currentJob.ID),
			slog.Int64("media_id", currentJob.MediaID),
			slog.String("job_type", string(currentJob.Type)),
		)

		if restoreState != nil {
			if err := restoreState(ctx, currentJob.MediaID, time.Now().UTC()); err != nil {
				logger.Error("recover media state failed", slog.Any("error", err))
				continue
			}
		}
		if err := p.jobs.Requeue(ctx, currentJob.ID, recoveryMessage, time.Now().UTC()); err != nil {
			logger.Error("requeue interrupted job failed", slog.Any("error", err))
			continue
		}

		logger.Warn("requeued interrupted job", slog.String("reason", recoveryMessage))
	}

	return nil
}

func toTranscriptSegments(items []ports.TranscriptionSegment) []transcript.Segment {
	segments := make([]transcript.Segment, 0, len(items))
	for _, item := range items {
		segments = append(segments, transcript.Segment{
			StartSec:   item.StartSec,
			EndSec:     item.EndSec,
			Text:       item.Text,
			Confidence: item.Confidence,
		})
	}

	return segments
}

func transcriptionDiagnostics(err error) string {
	transcriptionErr, ok := ports.AsTranscriptionError(err)
	if !ok {
		return ""
	}

	return strings.TrimSpace(transcriptionErr.Diagnostics)
}

type jobExecutionLog struct {
	startedAt time.Time
	logger    *slog.Logger
}

func newJobExecutionLog(logger *slog.Logger, attrs ...slog.Attr) jobExecutionLog {
	run := jobExecutionLog{
		startedAt: time.Now(),
		logger:    logger,
	}
	run.logger.Info("pipeline step started", attrsToAnySlice(attrs)...)
	return run
}

func (j jobExecutionLog) Success(attrs ...slog.Attr) {
	attrs = append(attrs, slog.Duration("duration", time.Since(j.startedAt)))
	j.logger.Info("pipeline step succeeded", attrsToAnySlice(attrs)...)
}

func (j jobExecutionLog) Failure(attrs ...slog.Attr) {
	attrs = append(attrs, slog.Duration("duration", time.Since(j.startedAt)))
	j.logger.Error("pipeline step failed", attrsToAnySlice(attrs)...)
}

func attrsToAnySlice(attrs []slog.Attr) []any {
	if len(attrs) == 0 {
		return nil
	}

	values := make([]any, 0, len(attrs))
	for _, attr := range attrs {
		values = append(values, attr)
	}

	return values
}

func buildUserFacingStageError(jobType job.Type, err error, diagnostics string) string {
	reason := compactDiagnostic(diagnostics, 240)
	if reason == "" {
		reason = compactDiagnostic(err.Error(), 240)
	}
	reason = humanizeUserReason(jobType, reason)
	if reason == "" {
		reason = "не удалось определить причину"
	}

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		switch jobType {
		case job.TypeExtractAudio, job.TypeExtractScreenshots:
			return "Истекло время ожидания ffmpeg."
		case job.TypeTranscribe:
			return "Истекло время ожидания распознавания текста."
		default:
			return "Истекло время ожидания обработки."
		}
	case jobType == job.TypeExtractAudio:
		return "Не удалось извлечь аудио: " + reason
	case jobType == job.TypeTranscribe:
		return "Не удалось распознать текст: " + reason
	case jobType == job.TypeAnalyzeTriggers:
		return "Не удалось проанализировать триггеры: " + reason
	case jobType == job.TypeExtractScreenshots:
		return "Не удалось подготовить скриншоты: " + reason
	case jobType == job.TypeGenerateSummary:
		return "Не удалось собрать саммари: " + reason
	default:
		return reason
	}
}

func humanizeUserReason(jobType job.Type, reason string) string {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return ""
	}
	if isUnhelpfulTranscriptionReason(trimmed) {
		return ""
	}

	lower := strings.ToLower(trimmed)
	if jobType == job.TypeTranscribe {
		switch {
		case strings.Contains(lower, "transcription backend returned empty text"):
			return "модель вернула пустой результат"
		case strings.Contains(lower, "no module named"):
			return "не удалось запустить Python-зависимости распознавания"
		case strings.Contains(lower, "out of memory"):
			return "не хватило памяти для запуска модели"
		case strings.Contains(lower, "cuda") && strings.Contains(lower, "not available"):
			return "CUDA недоступна для этой модели"
		case strings.Contains(lower, "exit status"):
			return "процесс распознавания завершился с ошибкой"
		}
	}

	if (jobType == job.TypeExtractAudio || jobType == job.TypeExtractScreenshots) && strings.Contains(lower, "exit status") {
		return "ffmpeg завершился с ошибкой"
	}

	return trimmed
}

func isUnhelpfulTranscriptionReason(reason string) bool {
	normalized := strings.TrimSpace(strings.ToLower(reason))
	switch normalized {
	case "python", "runtimeerror: python", "error: python":
		return true
	default:
		return false
	}
}

func buildInternalFailureMessage(prefix string, err error) string {
	return prefix + ": " + compactDiagnostic(err.Error(), 240)
}

func compactDiagnostic(raw string, limit int) string {
	lines := splitDiagnosticLines(raw)
	if len(lines) == 0 {
		return ""
	}

	for index := len(lines) - 1; index >= 0; index-- {
		if isDiagnosticNoise(lines[index]) {
			continue
		}
		return truncateMessage(lines[index], limit)
	}

	return truncateMessage(lines[len(lines)-1], limit)
}

func splitDiagnosticLines(raw string) []string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	parts := strings.Split(normalized, "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		line := strings.Join(strings.Fields(strings.TrimSpace(part)), " ")
		if line == "" {
			continue
		}
		line = strings.TrimPrefix(line, "stderr: ")
		line = strings.TrimPrefix(line, "stdout: ")
		lines = append(lines, line)
	}

	return lines
}

func isDiagnosticNoise(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	if lower == "" {
		return true
	}

	for _, prefix := range []string{
		"ffmpeg version",
		"built with",
		"configuration:",
		"libav",
		"input #",
		"metadata:",
		"stream #",
		"output #",
		"size=",
		"video:",
		"audio:",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}

	switch lower {
	case "conversion failed!", "[truncated]":
		return true
	}

	return false
}

func safeJoinBasePath(baseDir string, relativePath string) (string, error) {
	cleanRelativePath := filepath.Clean(filepath.FromSlash(relativePath))
	if cleanRelativePath == "." || cleanRelativePath == string(filepath.Separator) {
		return "", fmt.Errorf("invalid relative path %q", relativePath)
	}
	fullPath := filepath.Join(baseDir, cleanRelativePath)

	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve base dir: %w", err)
	}
	fullAbs, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("resolve full path: %w", err)
	}
	if fullAbs != baseAbs && !strings.HasPrefix(fullAbs, baseAbs+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes base dir", relativePath)
	}

	return fullAbs, nil
}
