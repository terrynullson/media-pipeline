package trigger

import (
	"testing"
	"time"

	"media-pipeline/internal/domain/transcript"
)

func TestDetectEvents_ContainsIsCaseInsensitiveAndDeterministic(t *testing.T) {
	t.Parallel()

	transcriptID := int64(77)
	events := DetectEvents(MatchInput{
		MediaID:      12,
		TranscriptID: &transcriptID,
		Segments: []transcript.Segment{
			{StartSec: 3, EndSec: 5, Text: "Customer asked for a REFUND today."},
		},
		Rules: []Rule{
			{ID: 1, Name: "Refund", Category: "billing", Pattern: "refund", MatchMode: MatchModeContains, Enabled: true},
		},
		CreatedAtUTC: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
	})

	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if events[0].MatchedText != "REFUND" {
		t.Fatalf("matched text = %q, want %q", events[0].MatchedText, "REFUND")
	}
	if events[0].SegmentIndex != 0 {
		t.Fatalf("segment index = %d, want 0", events[0].SegmentIndex)
	}
}

func TestDetectEvents_ExactMatchesWholeSegmentOnly(t *testing.T) {
	t.Parallel()

	events := DetectEvents(MatchInput{
		MediaID: 9,
		Segments: []transcript.Segment{
			{StartSec: 0, EndSec: 1, Text: "cancel my subscription"},
			{StartSec: 1, EndSec: 2, Text: "please cancel my subscription now"},
		},
		Rules: []Rule{
			{ID: 2, Name: "Cancel", Category: "retention", Pattern: "CANCEL MY SUBSCRIPTION", MatchMode: MatchModeExact, Enabled: true},
		},
		CreatedAtUTC: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
	})

	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if events[0].SegmentIndex != 0 {
		t.Fatalf("segment index = %d, want 0", events[0].SegmentIndex)
	}
}

func TestDetectEvents_BuildsNearbyContext(t *testing.T) {
	t.Parallel()

	events := DetectEvents(MatchInput{
		MediaID: 5,
		Segments: []transcript.Segment{
			{StartSec: 0, EndSec: 1, Text: "hello"},
			{StartSec: 1, EndSec: 2, Text: "please speak to a manager now"},
			{StartSec: 2, EndSec: 3, Text: "thank you"},
		},
		Rules: []Rule{
			{ID: 3, Name: "Escalation", Category: "support", Pattern: "speak to a manager", MatchMode: MatchModeContains, Enabled: true},
		},
		CreatedAtUTC: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
	})

	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if events[0].ContextText != "hello please speak to a manager now thank you" {
		t.Fatalf("context = %q, want nearby segment context", events[0].ContextText)
	}
}

func TestDetectEvents_ContainsDoesNotMatchInsideLargerWord(t *testing.T) {
	t.Parallel()

	events := DetectEvents(MatchInput{
		MediaID: 21,
		Segments: []transcript.Segment{
			{StartSec: 0, EndSec: 2, Text: "We discussed refundable credits."},
		},
		Rules: []Rule{
			{ID: 9, Name: "Refund", Category: "billing", Pattern: "fund", MatchMode: MatchModeContains, Enabled: true},
		},
		CreatedAtUTC: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
	})

	if len(events) != 0 {
		t.Fatalf("events = %d, want 0 for inner-word substring match", len(events))
	}
}
