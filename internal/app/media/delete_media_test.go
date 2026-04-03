package mediaapp

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"testing"

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
		},
	}
	uc := NewDeleteMediaUseCase(
		repo,
		stubDeleteStorage{err: errors.New("disk busy")},
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
		&stubMediaDeletionRepository{getErr: sql.ErrNoRows},
		stubDeleteStorage{},
		stubDeleteStorage{},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	if _, err := uc.Delete(context.Background(), 5); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Delete() error = %v, want sql.ErrNoRows", err)
	}
}

type stubMediaDeletionRepository struct {
	item    domainmedia.Media
	getErr  error
	delErr  error
	deleted bool
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

type stubDeleteStorage struct {
	err error
}

func (s stubDeleteStorage) Save(context.Context, string, io.Reader) (ports.StoredFile, error) {
	return ports.StoredFile{}, errors.New("not implemented")
}

func (s stubDeleteStorage) Delete(context.Context, string) error {
	return s.err
}
