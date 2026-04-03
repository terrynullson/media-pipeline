package trigger

import (
	"strings"
	"time"
	"unicode"

	"media-pipeline/internal/domain/transcript"
)

type MatchInput struct {
	MediaID      int64
	TranscriptID *int64
	Segments     []transcript.Segment
	Rules        []Rule
	CreatedAtUTC time.Time
}

func DetectEvents(input MatchInput) []Event {
	if len(input.Segments) == 0 || len(input.Rules) == 0 {
		return nil
	}

	events := make([]Event, 0)
	for segmentIndex, segment := range input.Segments {
		segmentText := strings.Join(strings.Fields(strings.TrimSpace(segment.Text)), " ")
		if segmentText == "" {
			continue
		}

		for _, rule := range input.Rules {
			rule = NormalizeRule(rule)
			if !rule.Enabled {
				continue
			}

			matchedText, ok := matchSegmentText(segmentText, rule)
			if !ok {
				continue
			}

			events = append(events, Event{
				MediaID:      input.MediaID,
				TranscriptID: input.TranscriptID,
				RuleID:       rule.ID,
				RuleName:     rule.Name,
				Category:     rule.Category,
				MatchedText:  matchedText,
				SegmentIndex: segmentIndex,
				StartSec:     segment.StartSec,
				EndSec:       segment.EndSec,
				SegmentText:  segmentText,
				ContextText:  buildContextText(input.Segments, segmentIndex),
				CreatedAtUTC: input.CreatedAtUTC,
			})
		}
	}

	return events
}

func matchSegmentText(segmentText string, rule Rule) (string, bool) {
	pattern := strings.Join(strings.Fields(strings.TrimSpace(rule.Pattern)), " ")
	if pattern == "" {
		return "", false
	}

	switch rule.MatchMode {
	case MatchModeExact:
		if strings.EqualFold(segmentText, pattern) {
			return segmentText, true
		}
		return "", false
	case MatchModeContains:
		return findContainsMatch(segmentText, pattern)
	default:
		return "", false
	}
}

func findContainsMatch(segmentText string, pattern string) (string, bool) {
	segmentRunes := []rune(segmentText)
	patternRunes := []rune(pattern)
	if len(patternRunes) == 0 || len(patternRunes) > len(segmentRunes) {
		return "", false
	}

	for start := 0; start <= len(segmentRunes)-len(patternRunes); start++ {
		end := start + len(patternRunes)
		candidate := string(segmentRunes[start:end])
		if strings.EqualFold(candidate, pattern) {
			if !hasPhraseBoundaries(segmentRunes, start, end) {
				continue
			}
			return candidate, true
		}
	}

	return "", false
}

func hasPhraseBoundaries(segmentRunes []rune, start int, end int) bool {
	if start > 0 && isWordRune(segmentRunes[start-1]) {
		return false
	}
	if end < len(segmentRunes) && isWordRune(segmentRunes[end]) {
		return false
	}

	return true
}

func isWordRune(value rune) bool {
	return unicode.IsLetter(value) || unicode.IsDigit(value)
}

func buildContextText(segments []transcript.Segment, currentIndex int) string {
	start := currentIndex - 1
	if start < 0 {
		start = 0
	}
	end := currentIndex + 1
	if end >= len(segments) {
		end = len(segments) - 1
	}

	parts := make([]string, 0, end-start+1)
	for idx := start; idx <= end; idx++ {
		text := strings.Join(strings.Fields(strings.TrimSpace(segments[idx].Text)), " ")
		if text == "" {
			continue
		}
		parts = append(parts, text)
	}

	return strings.Join(parts, " ")
}
