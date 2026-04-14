package mediaapp

import (
	"context"
	"sort"
	"time"

	"media-pipeline/internal/domain/transcript"
	"media-pipeline/internal/domain/trigger"
)

const triggerPreviewMaxTranscripts = 100
const triggerPreviewMaxMatches = 10

// previewTranscriptReader reads recent transcripts for preview.
type previewTranscriptReader interface {
	ListRecentWithSegments(ctx context.Context, limit int) ([]transcript.Transcript, error)
}

// TriggerPreviewRequest is the input for a dry-run trigger match.
type TriggerPreviewRequest struct {
	Pattern   string
	MatchMode string
}

// TriggerPreviewMediaMatch describes one media file that matched.
type TriggerPreviewMediaMatch struct {
	MediaID      int64   `json:"mediaId"`
	MatchCount   int     `json:"matchCount"`
	FirstMatchAt float64 `json:"firstMatchAt"`
}

// TriggerPreviewResult is the output of a dry-run trigger match.
type TriggerPreviewResult struct {
	TotalMatches int
	MediaMatches []TriggerPreviewMediaMatch
	Limited      bool
}

// TriggerPreviewUseCase runs a trigger rule against existing transcripts
// without saving anything.
type TriggerPreviewUseCase struct {
	transcriptRepo previewTranscriptReader
}

func NewTriggerPreviewUseCase(transcriptRepo previewTranscriptReader) *TriggerPreviewUseCase {
	return &TriggerPreviewUseCase{transcriptRepo: transcriptRepo}
}

func (u *TriggerPreviewUseCase) Preview(ctx context.Context, req TriggerPreviewRequest) (TriggerPreviewResult, error) {
	transcripts, err := u.transcriptRepo.ListRecentWithSegments(ctx, triggerPreviewMaxTranscripts)
	if err != nil {
		return TriggerPreviewResult{}, err
	}

	rule := trigger.Rule{
		ID:        0,
		Pattern:   req.Pattern,
		MatchMode: trigger.MatchMode(req.MatchMode),
		Enabled:   true,
	}

	// mediaID → match count + first match time
	type mediaAgg struct {
		count        int
		firstMatchAt float64
	}
	byMedia := make(map[int64]*mediaAgg)
	total := 0

	for _, t := range transcripts {
		events := trigger.DetectEvents(trigger.MatchInput{
			MediaID:  t.MediaID,
			Segments: t.Segments,
			Rules:    []trigger.Rule{rule},
			CreatedAtUTC: time.Now().UTC(),
		})
		if len(events) == 0 {
			continue
		}
		agg := byMedia[t.MediaID]
		if agg == nil {
			agg = &mediaAgg{firstMatchAt: events[0].StartSec}
			byMedia[t.MediaID] = agg
		}
		agg.count += len(events)
		total += len(events)
		for _, ev := range events {
			if ev.StartSec < agg.firstMatchAt {
				agg.firstMatchAt = ev.StartSec
			}
		}
	}

	matches := make([]TriggerPreviewMediaMatch, 0, len(byMedia))
	for mediaID, agg := range byMedia {
		matches = append(matches, TriggerPreviewMediaMatch{
			MediaID:      mediaID,
			MatchCount:   agg.count,
			FirstMatchAt: agg.firstMatchAt,
		})
	}
	// Sort by most matches first.
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].MatchCount > matches[j].MatchCount
	})

	limited := false
	if len(matches) > triggerPreviewMaxMatches {
		matches = matches[:triggerPreviewMaxMatches]
		limited = true
	}

	return TriggerPreviewResult{
		TotalMatches: total,
		MediaMatches: matches,
		Limited:      limited,
	}, nil
}
