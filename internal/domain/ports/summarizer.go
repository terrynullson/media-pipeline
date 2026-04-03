package ports

import (
	"context"

	"media-pipeline/internal/domain/transcript"
	domaintrigger "media-pipeline/internal/domain/trigger"
)

type SummaryInput struct {
	MediaID            int64
	Transcript         transcript.Transcript
	TriggerEvents      []domaintrigger.Event
	TriggerScreenshots []domaintrigger.Screenshot
}

type SummaryOutput struct {
	SummaryText string
	Highlights  []string
	Provider    string
}

type Summarizer interface {
	Generate(ctx context.Context, in SummaryInput) (SummaryOutput, error)
}
