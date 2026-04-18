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
	tracker  ImportTracker
	logger   *slog.Logger
}

type ImportTracker interface {
	Begin(ctx context.Context, key ImportKey, nowUTC time.Time) (BeginImportResult, error)
	MarkImported(ctx context.Context, key ImportKey, mediaID int64, nowUTC time.Time) error
	Delete(ctx context.Context, key ImportKey) error
}

func NewService(source Source, uploader Uploader, logger *slog.Logger) *Service {
	return &Service{
		source:   source,
		uploader: uploader,
		logger:   logger,
	}
}

func (s *Service) WithImportTracker(tracker ImportTracker) *Service {
	s.tracker = tracker
	return s
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

	key := ImportKey{
		RelativePath:  candidate.RelativePath,
		SizeBytes:     candidate.SizeBytes,
		ModifiedAtUTC: candidate.ModifiedAtUTC,
	}
	if s.tracker != nil {
		beginResult, err := s.tracker.Begin(ctx, key, nowUTC)
		if err != nil {
			return false, fmt.Errorf("begin auto-upload import tracking for %q: %w", candidate.RelativePath, err)
		}
		if !beginResult.Started {
			if beginResult.Status == ImportStatusImported {
				if err := s.source.MarkImported(ctx, candidate); err != nil {
					return false, fmt.Errorf("archive duplicate auto-upload candidate %q: %w", candidate.RelativePath, err)
				}
				logger.Info("auto-upload duplicate skipped and archived")
				return true, nil
			}
			logger.Debug("auto-upload candidate is already being tracked")
			return false, nil
		}
	}

	reader, err := s.source.Open(ctx, candidate)
	if err != nil {
		if s.tracker != nil {
			_ = s.tracker.Delete(context.Background(), key)
		}
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
		if s.tracker != nil {
			_ = s.tracker.Delete(context.Background(), key)
		}
		logger.Error("auto-upload import failed", slog.Any("error", err))
		return false, fmt.Errorf("import auto-upload candidate %q: %w", candidate.RelativePath, err)
	}
	if err := reader.Close(); err != nil {
		if s.tracker != nil {
			_ = s.tracker.Delete(context.Background(), key)
		}
		return false, fmt.Errorf("close auto-upload candidate %q: %w", candidate.RelativePath, err)
	}

	if s.tracker != nil {
		if err := s.tracker.MarkImported(ctx, key, result.MediaID, time.Now().UTC()); err != nil {
			return false, fmt.Errorf("mark auto-upload candidate %q as imported: %w", candidate.RelativePath, err)
		}
	}

	if err := s.source.MarkImported(ctx, candidate); err != nil {
		return false, fmt.Errorf("archive imported auto-upload candidate %q: %w", candidate.RelativePath, err)
	}

	logger.Info("auto-upload import completed", slog.Int64("media_id", result.MediaID))
	return true, nil
}
