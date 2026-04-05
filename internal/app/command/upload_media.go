package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/media"
	"media-pipeline/internal/domain/ports"
	"media-pipeline/internal/observability"
)

type MediaRepository interface {
	Create(ctx context.Context, m media.Media) (int64, error)
	Delete(ctx context.Context, id int64) error
	ListRecent(ctx context.Context, limit int) ([]media.Media, error)
}

type JobRepository interface {
	Create(ctx context.Context, j job.Job) (int64, error)
}

type UploadMediaInput struct {
	OriginalName        string
	MIMEType            string
	SizeBytes           int64
	Content             io.Reader
	StartedAtUTC        time.Time
	FinishedAtUTC       time.Time
	RuntimeSnapshotJSON string
}

type UploadMediaResult struct {
	MediaID int64
}

type UploadMediaUseCase struct {
	mediaRepo      MediaRepository
	jobRepo        JobRepository
	storage        ports.FileStorage
	maxUploadBytes int64
	logger         *slog.Logger
}

func NewUploadMediaUseCase(
	mediaRepo MediaRepository,
	jobRepo JobRepository,
	storage ports.FileStorage,
	maxUploadBytes int64,
	logger *slog.Logger,
) *UploadMediaUseCase {
	return &UploadMediaUseCase{
		mediaRepo:      mediaRepo,
		jobRepo:        jobRepo,
		storage:        storage,
		maxUploadBytes: maxUploadBytes,
		logger:         logger,
	}
}

func (u *UploadMediaUseCase) Upload(ctx context.Context, in UploadMediaInput) (UploadMediaResult, error) {
	logger := observability.LoggerFromContext(ctx, u.logger).With(
		slog.String("original_name", in.OriginalName),
		slog.Int64("declared_size_bytes", in.SizeBytes),
	)
	logger.Info("upload started")

	ext, bufferedContent, detectedMIMEType, err := validateUploadInput(in, u.maxUploadBytes)
	if err != nil {
		logger.Warn("upload validation failed", slog.Any("error", err))
		return UploadMediaResult{}, err
	}

	storedFile, err := u.storage.Save(ctx, in.OriginalName, bufferedContent)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			logger.Warn("upload canceled while saving file", slog.Any("error", err))
			return UploadMediaResult{}, fmt.Errorf("upload canceled: %w", err)
		}
		logger.Error("save uploaded file failed", slog.Any("error", err))
		return UploadMediaResult{}, fmt.Errorf("save uploaded file: %w", err)
	}

	nowUTC := time.Now().UTC()
	uploadStartedAt := in.StartedAtUTC.UTC()
	if uploadStartedAt.IsZero() {
		uploadStartedAt = nowUTC
	}
	uploadFinishedAt := in.FinishedAtUTC.UTC()
	if uploadFinishedAt.IsZero() {
		uploadFinishedAt = nowUTC
	}
	uploadDurationMS := uploadFinishedAt.Sub(uploadStartedAt).Milliseconds()
	if uploadDurationMS < 0 {
		uploadDurationMS = 0
	}
	mediaID, err := u.mediaRepo.Create(ctx, media.Media{
		OriginalName:        in.OriginalName,
		StoredName:          storedFile.StoredName,
		Extension:           ext,
		MIMEType:            firstNonEmpty(detectedMIMEType, in.MIMEType),
		SizeBytes:           storedFile.SizeBytes,
		StoragePath:         storedFile.RelativePath,
		RuntimeSnapshotJSON: in.RuntimeSnapshotJSON,
		Status:              media.StatusQueued,
		CreatedAtUTC:        nowUTC,
		UpdatedAtUTC:        nowUTC,
	})
	if err != nil {
		logger.Error("create media record failed",
			slog.Any("error", err),
			slog.String("storage_path", storedFile.RelativePath),
		)
		u.cleanupStoredFile(ctx, storedFile.RelativePath, "media create failure")
		return UploadMediaResult{}, fmt.Errorf("create media record: %w", err)
	}

	durationPtr := &uploadDurationMS
	startedAtPtr := &uploadStartedAt
	finishedAtPtr := &uploadFinishedAt
	_, err = u.jobRepo.Create(ctx, job.Job{
		MediaID:       mediaID,
		Type:          job.TypeUpload,
		Status:        job.StatusDone,
		Attempts:      0,
		ErrorMessage:  "",
		CreatedAtUTC:  uploadStartedAt,
		UpdatedAtUTC:  uploadFinishedAt,
		StartedAtUTC:  startedAtPtr,
		FinishedAtUTC: finishedAtPtr,
		DurationMS:    durationPtr,
	})
	if err != nil {
		logger.Error("create upload job record failed",
			slog.Any("error", err),
			slog.Int64("media_id", mediaID),
		)
		u.cleanupMediaRecord(ctx, mediaID)
		u.cleanupStoredFile(ctx, storedFile.RelativePath, "upload job create failure")
		return UploadMediaResult{}, fmt.Errorf("create upload job record: %w", err)
	}

	initialJobs := []job.Job{
		{
			MediaID:      mediaID,
			Type:         job.TypeExtractAudio,
			Status:       job.StatusPending,
			Attempts:     0,
			ErrorMessage: "",
			CreatedAtUTC: nowUTC,
			UpdatedAtUTC: nowUTC,
		},
	}
	mediaItem := media.Media{
		Extension: ext,
		MIMEType:  firstNonEmpty(detectedMIMEType, in.MIMEType),
	}
	if !mediaItem.IsAudioOnly() {
		initialJobs = append(initialJobs, job.Job{
			MediaID:      mediaID,
			Type:         job.TypePreparePreviewVideo,
			Status:       job.StatusPending,
			Attempts:     0,
			ErrorMessage: "",
			CreatedAtUTC: nowUTC,
			UpdatedAtUTC: nowUTC,
		})
	}

	for _, currentJob := range initialJobs {
		if _, err = u.jobRepo.Create(ctx, currentJob); err != nil {
			logger.Error("create job record failed",
				slog.Any("error", err),
				slog.Int64("media_id", mediaID),
				slog.String("job_type", string(currentJob.Type)),
				slog.String("storage_path", storedFile.RelativePath),
			)
			u.cleanupMediaRecord(ctx, mediaID)
			u.cleanupStoredFile(ctx, storedFile.RelativePath, "job create failure")
			return UploadMediaResult{}, fmt.Errorf("create job record: %w", err)
		}
	}

	logger.Info("upload completed",
		slog.Int64("media_id", mediaID),
		slog.String("stored_name", storedFile.StoredName),
		slog.String("storage_path", storedFile.RelativePath),
		slog.Int64("size_bytes", storedFile.SizeBytes),
		slog.String("mime_type", firstNonEmpty(detectedMIMEType, in.MIMEType)),
	)
	return UploadMediaResult{MediaID: mediaID}, nil
}

func (u *UploadMediaUseCase) ListRecent(ctx context.Context, limit int) ([]media.Media, error) {
	return u.mediaRepo.ListRecent(ctx, limit)
}

func (u *UploadMediaUseCase) cleanupStoredFile(ctx context.Context, relativePath, reason string) {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := u.storage.Delete(cleanupCtx, relativePath); err != nil {
		observability.LoggerFromContext(ctx, u.logger).Error(
			"cleanup stored file failed",
			slog.Any("error", err),
			slog.String("storage_path", relativePath),
			slog.String("reason", reason),
		)
	}
}

func (u *UploadMediaUseCase) cleanupMediaRecord(ctx context.Context, mediaID int64) {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := u.mediaRepo.Delete(cleanupCtx, mediaID); err != nil {
		observability.LoggerFromContext(ctx, u.logger).Error(
			"cleanup media record failed",
			slog.Any("error", err),
			slog.Int64("media_id", mediaID),
		)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
