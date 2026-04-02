package repositories

import (
	"context"
	"testing"
	"time"
)

func TestMediaRepository_StatusOnlyUpdatesPreservePathsAndTranscript(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	mediaRepo := NewMediaRepository(sqlDB)
	mediaID := createTestMedia(t, ctx, mediaRepo)
	nowUTC := time.Date(2026, 4, 3, 11, 0, 0, 0, time.UTC)

	if err := mediaRepo.MarkAudioExtracted(ctx, mediaID, "2026-04-03/media_1.wav", nowUTC); err != nil {
		t.Fatalf("MarkAudioExtracted() error = %v", err)
	}
	if err := mediaRepo.MarkTranscribed(ctx, mediaID, "hello world", nowUTC.Add(time.Minute)); err != nil {
		t.Fatalf("MarkTranscribed() error = %v", err)
	}
	if err := mediaRepo.MarkFailed(ctx, mediaID, nowUTC.Add(2*time.Minute)); err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}

	item, err := mediaRepo.GetByID(ctx, mediaID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if item.ExtractedAudioPath != "2026-04-03/media_1.wav" {
		t.Fatalf("ExtractedAudioPath = %q, want preserved value", item.ExtractedAudioPath)
	}
	if item.TranscriptText != "hello world" {
		t.Fatalf("TranscriptText = %q, want preserved value", item.TranscriptText)
	}
}
