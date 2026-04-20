package repositories

import (
	"context"
	"testing"
	"time"

	autouploadapp "media-pipeline/internal/app/autoupload"
	"media-pipeline/internal/domain/media"
)

func TestAutoUploadImportRepository_BeginAndMarkImported(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	repo := NewAutoUploadImportRepository(sqlDB)
	mediaRepo := NewMediaRepository(sqlDB)

	nowUTC := time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)
	key := autouploadapp.ImportKey{
		RelativePath:  "2026-04-04/source.mp4",
		SizeBytes:     4096,
		ModifiedAtUTC: nowUTC.Add(-time.Minute),
	}

	beginResult, err := repo.Begin(ctx, key, nowUTC)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if !beginResult.Started {
		t.Fatalf("Begin() Started = false, want true")
	}
	if beginResult.Status != autouploadapp.ImportStatusImporting {
		t.Fatalf("Begin() Status = %q, want %q", beginResult.Status, autouploadapp.ImportStatusImporting)
	}

	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName: "source.mp4",
		StoredName:   "source.mp4",
		Extension:    ".mp4",
		MIMEType:     "video/mp4",
		SizeBytes:    4096,
		StoragePath:  "2026-04-04/source.mp4",
		Status:       media.StatusUploaded,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}

	if err := repo.MarkImported(ctx, key, mediaID, nowUTC.Add(time.Minute)); err != nil {
		t.Fatalf("MarkImported() error = %v", err)
	}

	secondResult, err := repo.Begin(ctx, key, nowUTC.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("Begin(second) error = %v", err)
	}
	if secondResult.Started {
		t.Fatalf("Begin(second) Started = true, want false")
	}
	if secondResult.Status != autouploadapp.ImportStatusImported {
		t.Fatalf("Begin(second) Status = %q, want %q", secondResult.Status, autouploadapp.ImportStatusImported)
	}
	if secondResult.MediaID == nil || *secondResult.MediaID != mediaID {
		t.Fatalf("Begin(second) MediaID = %#v, want %d", secondResult.MediaID, mediaID)
	}
}

func TestAutoUploadImportRepository_DeleteAllowsRestart(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	repo := NewAutoUploadImportRepository(sqlDB)
	nowUTC := time.Date(2026, 4, 4, 11, 0, 0, 0, time.UTC)
	key := autouploadapp.ImportKey{
		RelativePath:  "2026-04-04/retry.mp4",
		SizeBytes:     2048,
		ModifiedAtUTC: nowUTC.Add(-2 * time.Minute),
	}

	beginResult, err := repo.Begin(ctx, key, nowUTC)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if !beginResult.Started {
		t.Fatalf("Begin() Started = false, want true")
	}

	if err := repo.Delete(ctx, key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	restarted, err := repo.Begin(ctx, key, nowUTC.Add(time.Minute))
	if err != nil {
		t.Fatalf("Begin(after delete) error = %v", err)
	}
	if !restarted.Started {
		t.Fatalf("Begin(after delete) Started = false, want true")
	}
	if restarted.Status != autouploadapp.ImportStatusImporting {
		t.Fatalf("Begin(after delete) Status = %q, want %q", restarted.Status, autouploadapp.ImportStatusImporting)
	}
}
