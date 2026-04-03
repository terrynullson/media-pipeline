package repositories

import (
	"context"
	"testing"
	"time"

	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/media"
	"media-pipeline/internal/domain/transcript"
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

func TestMediaRepository_DeleteWithAssociationsRemovesRows(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	mediaRepo := NewMediaRepository(sqlDB)
	jobRepo := NewJobRepository(sqlDB)
	transcriptRepo := NewTranscriptRepository(sqlDB)

	mediaID := createTestMedia(t, ctx, mediaRepo)
	nowUTC := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)

	if _, err := jobRepo.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeExtractAudio,
		Status:       job.StatusDone,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	}); err != nil {
		t.Fatalf("Create(job) error = %v", err)
	}

	if err := transcriptRepo.Save(ctx, transcript.Transcript{
		MediaID:      mediaID,
		Language:     "ru",
		FullText:     "hello world",
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
		Segments: []transcript.Segment{
			{StartSec: 0, EndSec: 1, Text: "hello"},
		},
	}); err != nil {
		t.Fatalf("Save(transcript) error = %v", err)
	}

	if err := mediaRepo.DeleteWithAssociations(ctx, mediaID); err != nil {
		t.Fatalf("DeleteWithAssociations() error = %v", err)
	}

	assertCount := func(query string, want int) {
		t.Helper()
		var count int
		if err := sqlDB.QueryRowContext(ctx, query, mediaID).Scan(&count); err != nil {
			t.Fatalf("QueryRow(%q) error = %v", query, err)
		}
		if count != want {
			t.Fatalf("count for %q = %d, want %d", query, count, want)
		}
	}

	assertCount("SELECT COUNT(*) FROM media WHERE id = ?", 0)
	assertCount("SELECT COUNT(*) FROM jobs WHERE media_id = ?", 0)
	assertCount("SELECT COUNT(*) FROM transcripts WHERE media_id = ?", 0)
	assertCount(`SELECT COUNT(*)
		FROM transcript_segments
		WHERE transcript_id IN (SELECT id FROM transcripts WHERE media_id = ?)`, 0)
}

func TestMediaRepository_PersistsRuntimeSnapshot(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	mediaRepo := NewMediaRepository(sqlDB)
	nowUTC := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName:        "snapshot.wav",
		StoredName:          "snapshot.wav",
		Extension:           ".wav",
		MIMEType:            "audio/wav",
		SizeBytes:           1024,
		StoragePath:         "2026-04-03/snapshot.wav",
		RuntimeSnapshotJSON: `{"request_ip":"127.0.0.1","user_agent":"test-agent"}`,
		Status:              media.StatusUploaded,
		CreatedAtUTC:        nowUTC,
		UpdatedAtUTC:        nowUTC,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}

	item, err := mediaRepo.GetByID(ctx, mediaID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if item.RuntimeSnapshotJSON == "" {
		t.Fatal("RuntimeSnapshotJSON = empty, want persisted value")
	}
}
