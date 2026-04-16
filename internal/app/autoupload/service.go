package autoupload

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"media-pipeline/internal/app/command"
	"media-pipeline/internal/observability"
)

type Uploader interface {
	Upload(ctx context.Context, in command.UploadMediaInput) (command.UploadMediaResult, error)
}

type Source interface {
	FindNext(ctx context.Context, nowUTC time.Time) (Candidate, bool, error)
	Open(ctx context.Context, candidate Candidate) (io.ReadCloser, error)
	MarkImported(ctx context.Context, candidate Candidate) error
}

type Candidate struct {
	Name          string
	RelativePath  string
	SizeBytes     int64
	ModifiedAtUTC time.Time
}

type Service struct {
	source   Source
	uploader Uploader
	logger   *slog.Logger
}

func NewService(source Source, uploader Uploader, logger *slog.Logger) *Service {
	return &Service{
		source:   source,
		uploader: uploader,
		logger:   logger,
	}
}

func (s *Service) ImportNext(ctx context.Context) (bool, error) {
	if s == nil || s.source == nil || s.uploader == nil {
		return false, nil
	}

	nowUTC := time.Now().UTC()
	candidate, ok, err := s.source.FindNext(ctx, nowUTC)
	if err != nil {
		return false, fmt.Errorf("find next auto-upload candidate: %w", err)
	}
	if !ok {
		return false, nil
	}

	logger := observability.LoggerFromContext(ctx, s.logger).With(
		slog.String("source", "auto_upload"),
		slog.String("candidate_name", candidate.Name),
		slog.String("candidate_path", candidate.RelativePath),
		slog.Int64("candidate_size_bytes", candidate.SizeBytes),
	)

	reader, err := s.source.Open(ctx, candidate)
	if err != nil {
		return false, fmt.Errorf("open auto-upload candidate %q: %w", candidate.RelativePath, err)
	}

	startedAtUTC := time.Now().UTC()
	result, err := s.uploader.Upload(ctx, command.UploadMediaInput{
		OriginalName: candidate.Name,
		SizeBytes:    candidate.SizeBytes,
		Content:      reader,
		StartedAtUTC: startedAtUTC,
	})
	if err != nil {
		_ = reader.Close()
		logger.Error("auto-upload import failed", slog.Any("error", err))
		return false, fmt.Errorf("import auto-upload candidate %q: %w", candidate.RelativePath, err)
	}
	if err := reader.Close(); err != nil {
		return false, fmt.Errorf("close auto-upload candidate %q: %w", candidate.RelativePath, err)
	}

	if err := s.source.MarkImported(ctx, candidate); err != nil {
		return false, fmt.Errorf("archive imported auto-upload candidate %q: %w", candidate.RelativePath, err)
	}

	logger.Info("auto-upload import completed", slog.Int64("media_id", result.MediaID))
	return true, nil
}
