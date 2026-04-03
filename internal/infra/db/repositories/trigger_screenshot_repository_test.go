package repositories

import (
	"context"
	"testing"
	"time"

	"media-pipeline/internal/domain/media"
	"media-pipeline/internal/domain/transcript"
	domaintrigger "media-pipeline/internal/domain/trigger"
)

func TestTriggerScreenshotRepository_ReplaceListAndPaths(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	mediaRepo := NewMediaRepository(sqlDB)
	transcriptRepo := NewTranscriptRepository(sqlDB)
	triggerRuleRepo := NewTriggerRuleRepository(sqlDB)
	triggerEventRepo := NewTriggerEventRepository(sqlDB)
	repo := NewTriggerScreenshotRepository(sqlDB)

	nowUTC := time.Date(2026, 4, 3, 18, 0, 0, 0, time.UTC)
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName: "sample.mp4",
		StoredName:   "sample.mp4",
		Extension:    ".mp4",
		MIMEType:     "video/mp4",
		SizeBytes:    4096,
		StoragePath:  "2026-04-03/sample.mp4",
		Status:       media.StatusTranscribed,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}
	if err := transcriptRepo.Save(ctx, transcript.Transcript{
		MediaID:      mediaID,
		Language:     "en",
		FullText:     "please refund",
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
		Segments: []transcript.Segment{
			{StartSec: 1, EndSec: 2, Text: "please refund"},
		},
	}); err != nil {
		t.Fatalf("Save(transcript) error = %v", err)
	}
	transcriptItem, ok, err := transcriptRepo.GetByMediaID(ctx, mediaID)
	if err != nil {
		t.Fatalf("GetByMediaID(transcript) error = %v", err)
	}
	if !ok {
		t.Fatal("GetByMediaID(transcript) ok = false, want true")
	}
	rules, err := triggerRuleRepo.List(ctx)
	if err != nil {
		t.Fatalf("List(trigger rules) error = %v", err)
	}
	if err := triggerEventRepo.ReplaceForMedia(ctx, mediaID, &transcriptItem.ID, []domaintrigger.Event{
		{
			MediaID:      mediaID,
			TranscriptID: &transcriptItem.ID,
			RuleID:       rules[0].ID,
			Category:     rules[0].Category,
			MatchedText:  "refund",
			SegmentIndex: 0,
			StartSec:     1,
			EndSec:       2,
			SegmentText:  "please refund",
			ContextText:  "please refund",
			CreatedAtUTC: nowUTC,
		},
	}); err != nil {
		t.Fatalf("ReplaceForMedia(trigger events) error = %v", err)
	}
	events, err := triggerEventRepo.ListByMediaID(ctx, mediaID)
	if err != nil {
		t.Fatalf("ListByMediaID(trigger events) error = %v", err)
	}

	if err := repo.ReplaceForMedia(ctx, mediaID, []domaintrigger.Screenshot{
		{
			MediaID:        mediaID,
			TriggerEventID: events[0].ID,
			TimestampSec:   1,
			ImagePath:      "2026-04-03/media_1_trigger_1_1000ms.jpg",
			Width:          640,
			Height:         360,
			CreatedAtUTC:   nowUTC,
		},
	}); err != nil {
		t.Fatalf("ReplaceForMedia(trigger screenshots) error = %v", err)
	}

	items, err := repo.ListByMediaID(ctx, mediaID)
	if err != nil {
		t.Fatalf("ListByMediaID() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].ImagePath != "2026-04-03/media_1_trigger_1_1000ms.jpg" {
		t.Fatalf("image_path = %q, want stored path", items[0].ImagePath)
	}

	paths, err := repo.ListPathsByMediaID(ctx, mediaID)
	if err != nil {
		t.Fatalf("ListPathsByMediaID() error = %v", err)
	}
	if len(paths) != 1 || paths[0] != "2026-04-03/media_1_trigger_1_1000ms.jpg" {
		t.Fatalf("paths = %#v, want screenshot path", paths)
	}
}
