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
}

type TranscriptionProfileProvider interface {
	GetCurrent(ctx context.Context) (transcription.Profile, error)
}

type Processor struct {
	jobs              JobRepository
	media             MediaRepository
	transcripts       TranscriptRepository
	triggerRules      TriggerRuleRepository
	triggerEvents     TriggerEventRepository
	audioExtractor    ports.AudioExtractor
	transcriber       ports.Transcriber
	profiles          TranscriptionProfileProvider
	logger            *slog.Logger
	uploadDir         string
	audioDir          string
	ffmpegTimeout     time.Duration
	transcribeTimeout time.Duration
}

func NewProcessor(
	jobRepo JobRepository,
	mediaRepo MediaRepository,
	transcriptRepo TranscriptRepository,
	triggerRuleRepo TriggerRuleRepository,
	triggerEventRepo TriggerEventRepository,
	audioExtractor ports.AudioExtractor,
	transcriber ports.Transcriber,
	profiles TranscriptionProfileProvider,
	uploadDir string,
	audioDir string,
	ffmpegTimeout time.Duration,
	transcribeTimeout time.Duration,
	logger *slog.Logger,
) *Processor {
	return &Processor{
		jobs:              jobRepo,
		media:             mediaRepo,
		transcripts:       transcriptRepo,
		triggerRules:      triggerRuleRepo,
		triggerEvents:     triggerEventRepo,
		audioExtractor:    audioExtractor,
		transcriber:       transcriber,
		profiles:          profiles,
		logger:            logger,
		uploadDir:         uploadDir,
		audioDir:          audioDir,
		ffmpegTimeout:     ffmpegTimeout,
		transcribeTimeout: transcribeTimeout,
	}
}

func (p *Processor) ProcessNext(ctx context.Context) (bool, error) {
	for _, jobType := range []job.Type{job.TypeExtractAudio, job.TypeTranscribe, job.TypeAnalyzeTriggers} {
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
	logger.Info("picked job")

	switch claimedJob.Type {
	case job.TypeExtractAudio:
		p.processExtractAudioJob(ctx, claimedJob, logger)
	case job.TypeTranscribe:
		p.processTranscribeJob(ctx, claimedJob, logger)
	case job.TypeAnalyzeTriggers:
		p.processAnalyzeTriggersJob(ctx, claimedJob, logger)
	default:
		p.failJob(ctx, claimedJob, 0, fmt.Sprintf("unsupported job type %q", claimedJob.Type), true, logger)
	}

	return true, nil
}

func (p *Processor) processExtractAudioJob(ctx context.Context, claimedJob job.Job, logger *slog.Logger) {
	mediaItem, err := p.media.GetByID(ctx, claimedJob.MediaID)
	if err != nil {
		failureMessage := fmt.Sprintf("load media %d: %v", claimedJob.MediaID, err)
		p.failJob(ctx, claimedJob, 0, failureMessage, true, logger)
		return
	}

	if err := p.media.MarkProcessing(ctx, mediaItem.ID, time.Now().UTC()); err != nil {
		failureMessage := fmt.Sprintf("mark media %d processing: %v", mediaItem.ID, err)
		p.failJob(ctx, claimedJob, mediaItem.ID, failureMessage, true, logger)
		return
	}

	inputPath, err := safeJoinBasePath(p.uploadDir, mediaItem.StoragePath)
	if err != nil {
		failureMessage := fmt.Sprintf("resolve input path: %v", err)
		p.failJob(ctx, claimedJob, mediaItem.ID, failureMessage, true, logger)
		return
	}

	ffmpegCtx, cancel := context.WithTimeout(ctx, p.ffmpegTimeout)
	defer cancel()

	logger.Info("starting ffmpeg",
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
		logger.Error("ffmpeg failed",
			slog.Any("error", err),
			slog.String("stderr", strings.TrimSpace(extractResult.Stderr)),
		)
		failureMessage := buildFailureMessage("extract audio", err, extractResult.Stderr)
		p.failJob(ctx, claimedJob, mediaItem.ID, failureMessage, true, logger)
		return
	}
	logger.Info("ffmpeg completed",
		slog.String("audio_path", extractResult.OutputPath),
		slog.String("stderr", strings.TrimSpace(extractResult.Stderr)),
	)

	if err := p.media.MarkAudioExtracted(ctx, mediaItem.ID, extractResult.OutputPath, time.Now().UTC()); err != nil {
		_ = cleanupOutputFile(p.audioDir, extractResult.OutputPath)
		failureMessage := fmt.Sprintf("persist extracted audio path: %v", err)
		p.failJob(ctx, claimedJob, mediaItem.ID, failureMessage, true, logger)
		return
	}

	if err := p.enqueueTranscribeJob(ctx, mediaItem.ID); err != nil {
		failureMessage := fmt.Sprintf("enqueue transcribe job: %v", err)
		p.failJob(ctx, claimedJob, mediaItem.ID, failureMessage, true, logger)
		return
	}

	if err := p.jobs.MarkDone(ctx, claimedJob.ID, time.Now().UTC()); err != nil {
		_ = cleanupOutputFile(p.audioDir, extractResult.OutputPath)
		failureMessage := fmt.Sprintf("mark job done: %v", err)
		p.failJob(ctx, claimedJob, mediaItem.ID, failureMessage, true, logger)
		return
	}

	logger.Info("job completed",
		slog.String("audio_path", extractResult.OutputPath),
	)
}

func (p *Processor) processTranscribeJob(ctx context.Context, claimedJob job.Job, logger *slog.Logger) {
	mediaItem, err := p.media.GetByID(ctx, claimedJob.MediaID)
	if err != nil {
		failureMessage := fmt.Sprintf("load media %d: %v", claimedJob.MediaID, err)
		p.failJob(ctx, claimedJob, 0, failureMessage, true, logger)
		return
	}

	if strings.TrimSpace(mediaItem.ExtractedAudioPath) == "" {
		failureMessage := fmt.Sprintf("media %d has empty extracted audio path", mediaItem.ID)
		p.failJob(ctx, claimedJob, mediaItem.ID, failureMessage, true, logger)
		return
	}

	if err := p.media.MarkTranscribing(ctx, mediaItem.ID, time.Now().UTC()); err != nil {
		failureMessage := fmt.Sprintf("mark media %d transcribing: %v", mediaItem.ID, err)
		p.failJob(ctx, claimedJob, mediaItem.ID, failureMessage, true, logger)
		return
	}

	audioPath, err := safeJoinBasePath(p.audioDir, mediaItem.ExtractedAudioPath)
	if err != nil {
		failureMessage := fmt.Sprintf("resolve audio path: %v", err)
		p.failJob(ctx, claimedJob, mediaItem.ID, failureMessage, true, logger)
		return
	}

	transcribeCtx, cancel := context.WithTimeout(ctx, p.transcribeTimeout)
	defer cancel()

	settings, err := p.resolveTranscriptionSettings(ctx, claimedJob, logger)
	if err != nil {
		failureMessage := fmt.Sprintf("resolve transcription settings: %v", err)
		p.failJob(ctx, claimedJob, mediaItem.ID, failureMessage, true, logger)
		return
	}

	logger.Info("starting transcription",
		slog.String("audio_path", audioPath),
		slog.String("backend", string(settings.Backend)),
		slog.String("model_name", settings.ModelName),
		slog.String("device", settings.Device),
		slog.String("compute_type", settings.ComputeType),
		slog.String("language", settings.Language),
		slog.Int("beam_size", settings.BeamSize),
		slog.Bool("vad_enabled", settings.VADEnabled),
		slog.Duration("timeout", p.transcribeTimeout),
	)

	result, err := p.transcriber.Transcribe(transcribeCtx, ports.TranscribeInput{
		AudioPath: audioPath,
		Settings:  settings,
	})
	if err != nil {
		diagnostics := transcriptionDiagnostics(err)
		if errors.Is(transcribeCtx.Err(), context.DeadlineExceeded) {
			logger.Error("transcription timeout",
				slog.String("audio_path", audioPath),
				slog.String("diagnostics", diagnostics),
			)
		} else {
			logger.Error("transcription failed",
				slog.Any("error", err),
				slog.String("diagnostics", diagnostics),
			)
		}

		failureMessage := buildFailureMessage("transcribe audio", err, diagnostics)
		p.failJob(ctx, claimedJob, mediaItem.ID, failureMessage, true, logger)
		return
	}

	logger.Info("transcription completed",
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
		logger.Error("transcript persistence failed", slog.Any("error", err))
		failureMessage := fmt.Sprintf("persist transcript: %v", err)
		p.failJob(ctx, claimedJob, mediaItem.ID, failureMessage, true, logger)
		return
	}
	logger.Info("transcript persisted successfully", slog.Int64("media_id", mediaItem.ID))

	if err := p.media.MarkTranscribed(ctx, mediaItem.ID, result.FullText, time.Now().UTC()); err != nil {
		failureMessage := fmt.Sprintf("mark media transcribed: %v", err)
		p.failJob(ctx, claimedJob, mediaItem.ID, failureMessage, true, logger)
		return
	}

	if err := p.enqueueNextJob(ctx, mediaItem.ID, job.TypeAnalyzeTriggers); err != nil {
		failureMessage := fmt.Sprintf("enqueue analyze triggers job: %v", err)
		p.failJob(ctx, claimedJob, 0, failureMessage, false, logger)
		return
	}

	if err := p.jobs.MarkDone(ctx, claimedJob.ID, time.Now().UTC()); err != nil {
		failureMessage := fmt.Sprintf("mark job done: %v", err)
		p.failJob(ctx, claimedJob, 0, failureMessage, false, logger)
		return
	}

	logger.Info("job completed", slog.Int("segments", len(result.Segments)))
}

func (p *Processor) processAnalyzeTriggersJob(ctx context.Context, claimedJob job.Job, logger *slog.Logger) {
	transcriptItem, ok, err := p.transcripts.GetByMediaID(ctx, claimedJob.MediaID)
	if err != nil {
		failureMessage := fmt.Sprintf("load transcript for media %d: %v", claimedJob.MediaID, err)
		p.failJob(ctx, claimedJob, 0, failureMessage, false, logger)
		return
	}
	if !ok {
		failureMessage := fmt.Sprintf("transcript for media %d was not found", claimedJob.MediaID)
		p.failJob(ctx, claimedJob, 0, failureMessage, false, logger)
		return
	}

	rules, err := p.triggerRules.ListEnabled(ctx)
	if err != nil {
		failureMessage := fmt.Sprintf("load enabled trigger rules: %v", err)
		p.failJob(ctx, claimedJob, 0, failureMessage, false, logger)
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
		failureMessage := fmt.Sprintf("persist trigger events: %v", err)
		p.failJob(ctx, claimedJob, 0, failureMessage, false, logger)
		return
	}

	if err := p.jobs.MarkDone(ctx, claimedJob.ID, nowUTC); err != nil {
		failureMessage := fmt.Sprintf("mark job done: %v", err)
		p.failJob(ctx, claimedJob, 0, failureMessage, false, logger)
		return
	}

	logger.Info("trigger analysis completed", slog.Int("events", len(events)))
}

func (p *Processor) RecoverInterruptedJobs(ctx context.Context) error {
	for _, recovery := range []struct {
		jobType      job.Type
		restoreState func(context.Context, int64, time.Time) error
	}{
		{jobType: job.TypeExtractAudio, restoreState: p.media.MarkUploaded},
		{jobType: job.TypeTranscribe, restoreState: p.media.MarkAudioReady},
		{jobType: job.TypeAnalyzeTriggers},
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
	logger *slog.Logger,
) {
	logger.Error("job failed", slog.String("reason", failureMessage))

	if markMediaFailed && mediaID > 0 {
		if err := p.media.MarkFailed(ctx, mediaID, time.Now().UTC()); err != nil {
			logger.Error("mark media failed", slog.Any("error", err))
		}
	}

	if err := p.jobs.MarkFailed(ctx, currentJob.ID, truncateMessage(failureMessage, 2000), time.Now().UTC()); err != nil {
		logger.Error("mark job failed", slog.Any("error", err))
	}
}

func buildFailureMessage(action string, err error, stderr string) string {
	var builder strings.Builder
	builder.WriteString(action)
	builder.WriteString(": ")
	builder.WriteString(err.Error())

	trimmedStderr := strings.TrimSpace(stderr)
	if trimmedStderr != "" {
		builder.WriteString(" | stderr: ")
		builder.WriteString(trimmedStderr)
	}

	return builder.String()
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
