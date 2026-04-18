package mediaapp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"media-pipeline/internal/domain/job"
	domainmedia "media-pipeline/internal/domain/media"
	"media-pipeline/internal/domain/ports"
)

var ErrMediaBusy = errors.New("media is currently processing")
var ErrMediaCancelTimeout = errors.New("media cancellation timed out")

type MediaDeletionRepository interface {
	GetByID(ctx context.Context, id int64) (domainmedia.Media, error)
	DeleteWithAssociations(ctx context.Context, id int64) error
}

type MediaDeletionJobReader interface {
	ListByMediaID(ctx context.Context, mediaID int64) ([]job.Job, error)
}

type ScreenshotPathReader interface {
	ListPathsByMediaID(ctx context.Context, mediaID int64) ([]string, error)
}

type MediaCancellationRequester interface {
	Request(ctx context.Context, mediaID int64, requestedAtUTC time.Time) error
	Delete(ctx context.Context, mediaID int64) error
}

type DeleteMediaResult struct {
	MediaID         int64
	CleanupWarnings []string
}

type DeleteMediaUseCase struct {
	repo              MediaDeletionRepository
	jobReader         MediaDeletionJobReader
	cancelRequester   MediaCancellationRequester
	screenshots       ScreenshotPathReader
	uploadStorage     ports.FileStorage
	audioStorage      ports.FileStorage
	previewStorage    ports.FileStorage
	screenshotStorage ports.FileStorage
	logger            *slog.Logger
}

func NewDeleteMediaUseCase(
	repo MediaDeletionRepository,
	jobReader MediaDeletionJobReader,
	cancelRequester MediaCancellationRequester,
	screenshots ScreenshotPathReader,
	uploadStorage ports.FileStorage,
	audioStorage ports.FileStorage,
	previewStorage ports.FileStorage,
	screenshotStorage ports.FileStorage,
	logger *slog.Logger,
) *DeleteMediaUseCase {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &DeleteMediaUseCase{
		repo:              repo,
		jobReader:         jobReader,
		cancelRequester:   cancelRequester,
		screenshots:       screenshots,
		uploadStorage:     uploadStorage,
		audioStorage:      audioStorage,
		previewStorage:    previewStorage,
		screenshotStorage: screenshotStorage,
		logger:            logger,
	}
}

func (u *DeleteMediaUseCase) Delete(ctx context.Context, mediaID int64) (DeleteMediaResult, error) {
	mediaItem, err := u.repo.GetByID(ctx, mediaID)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return DeleteMediaResult{}, fmt.Errorf("media %d not found: %w", mediaID, ports.ErrNotFound)
		}
		return DeleteMediaResult{}, fmt.Errorf("load media for deletion: %w", err)
	}

	jobs, err := u.jobReader.ListByMediaID(ctx, mediaID)
	if err != nil {
		return DeleteMediaResult{}, fmt.Errorf("load media jobs for deletion: %w", err)
	}
	if hasRunningJobs(jobs) {
		if u.cancelRequester == nil {
			return DeleteMediaResult{}, fmt.Errorf("media %d is currently processing: %w", mediaID, ErrMediaBusy)
		}
		if err := u.cancelRequester.Request(ctx, mediaID, time.Now().UTC()); err != nil {
			return DeleteMediaResult{}, fmt.Errorf("request media cancellation: %w", err)
		}
		if err := u.waitUntilStopped(ctx, mediaID); err != nil {
			return DeleteMediaResult{}, err
		}
		jobs, err = u.jobReader.ListByMediaID(ctx, mediaID)
		if err != nil {
			return DeleteMediaResult{}, fmt.Errorf("reload media jobs for deletion: %w", err)
		}
	}

	screenshotPaths := make([]string, 0)
	if u.screenshots != nil {
		screenshotPaths, err = u.screenshots.ListPathsByMediaID(ctx, mediaID)
		if err != nil {
			return DeleteMediaResult{}, fmt.Errorf("load screenshot paths for deletion: %w", err)
		}
	}

	if err := u.repo.DeleteWithAssociations(ctx, mediaID); err != nil {
		return DeleteMediaResult{}, fmt.Errorf("delete media %d and associations: %w", mediaID, err)
	}
	if u.cancelRequester != nil {
		_ = u.cancelRequester.Delete(context.Background(), mediaID)
	}

	result := DeleteMediaResult{MediaID: mediaID}
	result.CleanupWarnings = append(result.CleanupWarnings, u.cleanupFile(mediaID, "uploaded media", mediaItem.StoragePath, u.uploadStorage)...)
	result.CleanupWarnings = append(result.CleanupWarnings, u.cleanupFile(mediaID, "extracted audio", mediaItem.ExtractedAudioPath, u.audioStorage)...)
	result.CleanupWarnings = append(result.CleanupWarnings, u.cleanupFile(mediaID, "preview video", mediaItem.PreviewVideoPath, u.previewStorage)...)
	for _, screenshotPath := range screenshotPaths {
		result.CleanupWarnings = append(result.CleanupWarnings, u.cleanupFile(mediaID, "trigger screenshot", screenshotPath, u.screenshotStorage)...)
	}

	return result, nil
}

func (u *DeleteMediaUseCase) waitUntilStopped(ctx context.Context, mediaID int64) error {
	waitCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		jobs, err := u.jobReader.ListByMediaID(waitCtx, mediaID)
		if err != nil {
			return fmt.Errorf("poll media jobs for cancellation: %w", err)
		}
		if !hasRunningJobs(jobs) {
			return nil
		}

		select {
		case <-waitCtx.Done():
			return fmt.Errorf("media %d is still processing after cancellation request: %w", mediaID, ErrMediaCancelTimeout)
		case <-ticker.C:
		}
	}
}

func hasRunningJobs(jobs []job.Job) bool {
	for _, currentJob := range jobs {
		if currentJob.Status == job.StatusRunning {
			return true
		}
	}
	return false
}

func (u *DeleteMediaUseCase) cleanupFile(mediaID int64, label string, relativePath string, storage ports.FileStorage) []string {
	if storage == nil || relativePath == "" {
		return nil
	}

	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := storage.Delete(cleanupCtx, relativePath); err != nil {
		message := fmt.Sprintf("could not remove %s file %q", label, relativePath)
		u.logger.Warn(
			"media file cleanup failed",
			slog.Int64("media_id", mediaID),
			slog.String("file_kind", label),
			slog.String("relative_path", relativePath),
			slog.Any("error", err),
		)
		return []string{message}
	}

	return nil
}
