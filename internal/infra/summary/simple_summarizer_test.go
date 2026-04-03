package summary

import (
	"context"
	"strings"
	"testing"

	"media-pipeline/internal/domain/ports"
	"media-pipeline/internal/domain/transcript"
	domaintrigger "media-pipeline/internal/domain/trigger"
)

func TestSimpleSummarizer_GenerateIncludesTriggersAndScreenshots(t *testing.T) {
	t.Parallel()

	summarizer := NewSimpleSummarizer()
	out, err := summarizer.Generate(context.Background(), ports.SummaryInput{
		MediaID: 10,
		Transcript: transcript.Transcript{
			MediaID:  10,
			FullText: "Клиент рассказал о проблеме с заказом. Затем попросил возврат средств. Менеджер уточнил детали.",
		},
		TriggerEvents: []domaintrigger.Event{
			{MatchedText: "refund", Category: "billing"},
		},
		TriggerScreenshots: []domaintrigger.Screenshot{
			{TriggerEventID: 1},
		},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if out.Provider != ProviderSimple {
		t.Fatalf("Provider = %q, want %q", out.Provider, ProviderSimple)
	}
	if !strings.Contains(out.SummaryText, "refund") {
		t.Fatalf("SummaryText = %q, want trigger mention", out.SummaryText)
	}
	if len(out.Highlights) == 0 {
		t.Fatal("Highlights = 0, want non-empty highlights")
	}
}
