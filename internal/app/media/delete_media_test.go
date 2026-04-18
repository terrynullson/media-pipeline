package mediaapp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"media-pipeline/internal/domain/job"
	domainmedia "media-pipeline/internal/domain/media"
	"media-pipeline/internal/domain/ports"
)

func TestDeleteMediaUseCase_DeleteRemovesRecordAndCollectsWarnings(t *testing.T) {
	t.Parallel()

	repo := &stubMediaDeletionRepository{
		item: domainmedia.Media{
			ID:                 9,
			StoragePath:        "uploads/demo.wav",
			ExtractedAudioPath: "audio/demo.wav",
			PreviewVideoPath:   "preview/demo.mp4",
		},
	}
	uc := NewDeleteMediaUseCase(
		repo,
		repo,
		&stubMediaCancellationRequester{},
		stubScreenshotPathReader{},
		stubDeleteStorage{err: errors.New("disk busy")},
		stubDeleteStorage{},
		stubDeleteStorage{},
		stubDeleteStorage{},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	result, err := uc.Delete(context.Background(), 9)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !repo.deleted {
		t.Fatal("DeleteWithAssociations() was not called")
	}
	if len(result.CleanupWarnings) != 1 {
		t.Fatalf("CleanupWarnings = %#v, want one warning", result.CleanupWarnings)
	}
}

func TestDeleteMediaUseCase_DeleteReturnsNotFound(t *testing.T) {
	t.Parallel()

	uc := NewDeleteMediaUseCase(
		&stubMediaDeletionRepository{getErr: ports.ErrNotFound},
		&stubMediaDeletionRepository{getErr: ports.ErrNotFound},
		&stubMediaCancellationRequester{},
		stubScreenshotPathReader{},
		stubDeleteStorage{},
		stubDeleteStorage{},
		stubDeleteStorage{},
		stubDeleteStorage{},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	if _, err := uc.Delete(context.Background(), 5); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete() error = %v, want ports.ErrNotFound", err)
	}
}

func TestDeleteMediaUseCase_DeleteReturnsBusyWhenJobIsRunningAndCancellationUnavailable(t *testing.T) {
	t.Parallel()

	repo := &stubMediaDeletionRepository{
		item: domainmedia.Media{ID: 12},
		jobs: []job.Job{{ID: 99, MediaID: 12, Status: job.StatusRunning, Type: job.TypePreparePreviewVideo}},
	}

	uc := NewDeleteMediaUseCase(
		repo,
		repo,
		nil,
		stubScreenshotPathReader{},
		stubDeleteStorage{},
		stubDeleteStorage{},
		stubDeleteStorage{},
		stubDeleteStorage{},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	if _, err := uc.Delete(context.Background(), 12); !errors.Is(err, ErrMediaBusy) {
		t.Fatalf("Delete() error = %v, want ErrMediaBusy", err)
	}
}

func TestDeleteMediaUseCase_DeleteRequestsCancellationBeforeRemovingRecord(t *testing.T) {
	t.Parallel()

	repo := &stubMediaDeletionRepository{
		item: domainmedia.Media{ID: 15, StoragePath: "uploads/demo.mp4"},
		jobs: []job.Job{
			{ID: 101, MediaID: 15, Status: job.StatusRunning, Type: job.TypePreparePreviewVideo},
			{ID: 102, MediaID: 15, Status: job.StatusDone, Type: job.TypeUpload},
		},
	}
	cancelRequester := &stubMediaCancellationRequester{}

	uc := NewDeleteMediaUseCase(
		repo,
		repo,
		cancelRequester,
		stubScreenshotPathReader{},
		stubDeleteStorage{},
		stubDeleteStorage{},
		stubDeleteStorage{},
		stubDeleteStorage{},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	result, err := uc.Delete(context.Background(), 15)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if result.MediaID != 15 {
		t.Fatalf("MediaID = %d, want 15", result.MediaID)
	}
	if !repo.deleted {
		t.Fatal("DeleteWithAssociations() was not called")
	}
	if len(cancelRequester.requested) != 1 || cancelRequester.requested[0] != 15 {
		t.Fatalf("requested = %#v, want [15]", cancelRequester.requested)
	}
	if len(cancelRequester.deleted) != 1 || cancelRequester.deleted[0] != 15 {
		t.Fatalf("deleted = %#v, want [15]", cancelRequester.deleted)
	}
}

type stubMediaDeletionRepository struct {
	item    domainmedia.Media
	getErr  error
	delErr  error
	deleted bool
	jobs    []job.Job
	polls   int
}

func (s *stubMediaDeletionRepository) GetByID(context.Context, int64) (domainmedia.Media, error) {
	if s.getErr != nil {
		return domainmedia.Media{}, s.getErr
	}
	return s.item, nil
}

func (s *stubMediaDeletionRepository) DeleteWithAssociations(context.Context, int64) error {
	s.deleted = true
	return s.delErr
}

func (s *stubMediaDeletionRepository) ListByMediaID(context.Context, int64) ([]job.Job, error) {
	s.polls++
	if s.polls > 1 {
		items := make([]job.Job, 0, len(s.jobs))
		for _, current := range s.jobs {
			if current.Status == job.StatusRunning {
				current.Status = job.StatusDone
			}
			items = append(items, current)
		}
		return items, nil
	}
	return s.jobs, nil
}

type stubDeleteStorage struct {
	err error
}

type stubMediaCancellationRequester struct {
	requested []int64
	deleted   []int64
}

func (s *stubMediaCancellationRequester) Request(_ context.Context, mediaID int64, _ time.Time) error {
	s.requested = append(s.requested, mediaID)
	return nil
}

func (s *stubMediaCancellationRequester) Delete(_ context.Context, mediaID int64) error {
	s.deleted = append(s.deleted, mediaID)
	return nil
}

type stubScreenshotPathReader struct {
	paths []string
	err   error
}

func (s stubScreenshotPathReader) ListPathsByMediaID(context.Context, int64) ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.paths, nil
}

func (s stubDeleteStorage) Save(context.Context, string, io.Reader) (ports.StoredFile, error) {
	return ports.StoredFile{}, errors.New("not implemented")
}

func (s stubDeleteStorage) Delete(context.Context, string) error {
	return s.err
}
