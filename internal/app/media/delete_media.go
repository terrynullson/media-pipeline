package mediaapp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	domainmedia "media-pipeline/internal/domain/media"
	"media-pipeline/internal/domain/ports"
)

type MediaDeletionRepository interface {
	GetByID(ctx context.Context, id int64) (domainmedia.Media, error)
	DeleteWithAssociations(ctx context.Context, id int64) error
}

type ScreenshotPathReader interface {
	ListPathsByMediaID(ctx context.Context, mediaID int64) ([]string, error)
}

type DeleteMediaResult struct {
	MediaID         int64
	CleanupWarnings []string
}

type DeleteMediaUseCase struct {
	repo              MediaDeletionRepository
	screenshots       ScreenshotPathReader
	uploadStorage     ports.FileStorage
	audioStorage      ports.FileStorage
	previewStorage    ports.FileStorage
	screenshotStorage ports.FileStorage
	logger            *slog.Logger
}

func NewDeleteMediaUseCase(
	repo MediaDeletionRepository,
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
		if errors.Is(err, sql.ErrNoRows) {
			return DeleteMediaResult{}, fmt.Errorf("media %d not found: %w", mediaID, err)
		}
		return DeleteMediaResult{}, fmt.Errorf("load media for deletion: %w", err)
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

	result := DeleteMediaResult{MediaID: mediaID}
	result.CleanupWarnings = append(result.CleanupWarnings, u.cleanupFile(mediaID, "uploaded media", mediaItem.StoragePath, u.uploadStorage)...)
	result.CleanupWarnings = append(result.CleanupWarnings, u.cleanupFile(mediaID, "extracted audio", mediaItem.ExtractedAudioPath, u.audioStorage)...)
	result.CleanupWarnings = append(result.CleanupWarnings, u.cleanupFile(mediaID, "preview video", mediaItem.PreviewVideoPath, u.previewStorage)...)
	for _, screenshotPath := range screenshotPaths {
		result.CleanupWarnings = append(result.CleanupWarnings, u.cleanupFile(mediaID, "trigger screenshot", screenshotPath, u.screenshotStorage)...)
	}

	return result, nil
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
