package repositories

import (
	"context"
	"testing"
	"time"

	"media-pipeline/internal/domain/media"
	"media-pipeline/internal/domain/transcript"
	domaintrigger "media-pipeline/internal/domain/trigger"
)

func TestTriggerEventRepository_ReplaceAndListByMediaID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	mediaRepo := NewMediaRepository(sqlDB)
	transcriptRepo := NewTranscriptRepository(sqlDB)
	triggerRuleRepo := NewTriggerRuleRepository(sqlDB)
	triggerEventRepo := NewTriggerEventRepository(sqlDB)

	nowUTC := time.Date(2026, 4, 3, 16, 0, 0, 0, time.UTC)
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName: "sample.wav",
		StoredName:   "sample.wav",
		Extension:    ".wav",
		MIMEType:     "audio/wav",
		SizeBytes:    2048,
		StoragePath:  "2026-04-03/sample.wav",
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
		FullText:     "please refund this order",
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
		Segments: []transcript.Segment{
			{StartSec: 0, EndSec: 2, Text: "please refund this order"},
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

	rule, err := triggerRuleRepo.Create(ctx, domaintrigger.Rule{
		Name:         "Manual Refund Rule",
		Category:     "billing",
		Pattern:      "refund",
		MatchMode:    domaintrigger.MatchModeContains,
		Enabled:      true,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	})
	if err != nil {
		t.Fatalf("Create(trigger rule) error = %v", err)
	}

	if err := triggerEventRepo.ReplaceForMedia(ctx, mediaID, &transcriptItem.ID, []domaintrigger.Event{
		{
			MediaID:      mediaID,
			TranscriptID: &transcriptItem.ID,
			RuleID:       rule.ID,
			Category:     "billing",
			MatchedText:  "refund",
			SegmentIndex: 0,
			StartSec:     0,
			EndSec:       2,
			SegmentText:  "please refund this order",
			ContextText:  "please refund this order",
			CreatedAtUTC: nowUTC,
		},
	}); err != nil {
		t.Fatalf("ReplaceForMedia() error = %v", err)
	}

	items, err := triggerEventRepo.ListByMediaID(ctx, mediaID)
	if err != nil {
		t.Fatalf("ListByMediaID() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].RuleName != "Manual Refund Rule" {
		t.Fatalf("rule name = %q, want %q", items[0].RuleName, "Manual Refund Rule")
	}
	if items[0].MatchedText != "refund" {
		t.Fatalf("matched text = %q, want refund", items[0].MatchedText)
	}

	counts, err := triggerEventRepo.CountByMediaIDs(ctx, []int64{mediaID, mediaID + 1})
	if err != nil {
		t.Fatalf("CountByMediaIDs() error = %v", err)
	}
	if counts[mediaID] != 1 {
		t.Fatalf("counts[%d] = %d, want 1", mediaID, counts[mediaID])
	}
}
