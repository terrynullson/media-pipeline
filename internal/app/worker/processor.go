package worker

import (
	"context"
	"database/sql"
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
)

type JobRepository interface {
	ClaimNextPending(ctx context.Context, jobType job.Type, nowUTC time.Time) (job.Job, bool, error)
	MarkDone(ctx context.Context, id int64, nowUTC time.Time) error
	MarkFailed(ctx context.Context, id int64, errorMessage string, nowUTC time.Time) error
}

type MediaRepository interface {
	GetByID(ctx context.Context, id int64) (media.Media, error)
	MarkProcessing(ctx context.Context, id int64, nowUTC time.Time) error
	MarkAudioExtracted(ctx context.Context, id int64, extractedAudioPath string, nowUTC time.Time) error
	MarkFailed(ctx context.Context, id int64, nowUTC time.Time) error
}

type Processor struct {
	jobs           JobRepository
	media          MediaRepository
	audioExtractor ports.AudioExtractor
	logger         *slog.Logger
	uploadDir      string
	audioDir       string
	ffmpegTimeout  time.Duration
}

func NewProcessor(
	jobRepo JobRepository,
	mediaRepo MediaRepository,
	audioExtractor ports.AudioExtractor,
	uploadDir string,
	audioDir string,
	ffmpegTimeout time.Duration,
	logger *slog.Logger,
) *Processor {
	return &Processor{
		jobs:           jobRepo,
		media:          mediaRepo,
		audioExtractor: audioExtractor,
		logger:         logger,
		uploadDir:      uploadDir,
		audioDir:       audioDir,
		ffmpegTimeout:  ffmpegTimeout,
	}
}

func (p *Processor) ProcessNext(ctx context.Context) (bool, error) {
	nowUTC := time.Now().UTC()
	claimedJob, ok, err := p.jobs.ClaimNextPending(ctx, job.TypeExtractAudio, nowUTC)
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

	mediaItem, err := p.media.GetByID(ctx, claimedJob.MediaID)
	if err != nil {
		failureMessage := fmt.Sprintf("load media %d: %v", claimedJob.MediaID, err)
		p.failJob(ctx, claimedJob, 0, failureMessage, logger)
		return true, nil
	}

	if err := p.media.MarkProcessing(ctx, mediaItem.ID, time.Now().UTC()); err != nil {
		failureMessage := fmt.Sprintf("mark media %d processing: %v", mediaItem.ID, err)
		p.failJob(ctx, claimedJob, mediaItem.ID, failureMessage, logger)
		return true, nil
	}

	inputPath, err := safeJoinBasePath(p.uploadDir, mediaItem.StoragePath)
	if err != nil {
		failureMessage := fmt.Sprintf("resolve input path: %v", err)
		p.failJob(ctx, claimedJob, mediaItem.ID, failureMessage, logger)
		return true, nil
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
		failureMessage := buildFailureMessage("extract audio", err, extractResult.Stderr)
		p.failJob(ctx, claimedJob, mediaItem.ID, failureMessage, logger)
		return true, nil
	}

	if err := p.media.MarkAudioExtracted(ctx, mediaItem.ID, extractResult.OutputPath, time.Now().UTC()); err != nil {
		_ = cleanupOutputFile(p.audioDir, extractResult.OutputPath)
		failureMessage := fmt.Sprintf("persist extracted audio path: %v", err)
		p.failJob(ctx, claimedJob, mediaItem.ID, failureMessage, logger)
		return true, nil
	}

	if err := p.jobs.MarkDone(ctx, claimedJob.ID, time.Now().UTC()); err != nil {
		_ = cleanupOutputFile(p.audioDir, extractResult.OutputPath)
		failureMessage := fmt.Sprintf("mark job done: %v", err)
		p.failJob(ctx, claimedJob, mediaItem.ID, failureMessage, logger)
		return true, nil
	}

	logger.Info("job completed",
		slog.String("audio_path", extractResult.OutputPath),
	)
	return true, nil
}

func (p *Processor) failJob(ctx context.Context, currentJob job.Job, mediaID int64, failureMessage string, logger *slog.Logger) {
	logger.Error("job failed", slog.String("reason", failureMessage))

	if mediaID > 0 {
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

func safeJoinBasePath(baseDir string, relativePath string) (string, error) {
	cleanRelativePath := filepath.Clean(filepath.FromSlash(relativePath))
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

func IsMissingMediaError(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
