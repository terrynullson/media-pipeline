package summary

import (
	"context"
	"fmt"
	"strings"

	"media-pipeline/internal/domain/ports"
	domaintrigger "media-pipeline/internal/domain/trigger"
)

const ProviderSimple = "simple-summary-v1"

type SimpleSummarizer struct{}

func NewSimpleSummarizer() *SimpleSummarizer {
	return &SimpleSummarizer{}
}

func (s *SimpleSummarizer) Generate(_ context.Context, in ports.SummaryInput) (ports.SummaryOutput, error) {
	fullText := normalizeSummaryText(in.Transcript.FullText)
	if fullText == "" {
		return ports.SummaryOutput{}, fmt.Errorf("summary source transcript is empty")
	}

	sentences := splitSentences(fullText)
	summaryText := buildSummaryText(sentences, in.TriggerEvents, len(in.TriggerScreenshots))
	highlights := buildHighlights(sentences, in.TriggerEvents, len(in.TriggerScreenshots))

	return ports.SummaryOutput{
		SummaryText: summaryText,
		Highlights:  highlights,
		Provider:    ProviderSimple,
	}, nil
}

func buildSummaryText(sentences []string, events []domaintrigger.Event, screenshotCount int) string {
	parts := make([]string, 0, 4)
	for _, sentence := range sentences {
		if strings.TrimSpace(sentence) == "" {
			continue
		}
		parts = append(parts, sentence)
		if len(parts) == 2 {
			break
		}
	}

	if len(events) > 0 {
		parts = append(parts, buildTriggerSentence(events, screenshotCount))
	} else if screenshotCount > 0 {
		parts = append(parts, fmt.Sprintf("Дополнительно подготовлено %d скриншот(ов) по материалу.", screenshotCount))
	}

	result := strings.Join(parts, " ")
	result = strings.Join(strings.Fields(strings.TrimSpace(result)), " ")
	if result == "" && len(sentences) > 0 {
		result = sentences[0]
	}
	if len(result) <= 480 {
		return result
	}

	return strings.TrimSpace(result[:477]) + "..."
}

func buildHighlights(sentences []string, events []domaintrigger.Event, screenshotCount int) []string {
	highlights := make([]string, 0, 4)
	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if sentence == "" {
			continue
		}
		highlights = append(highlights, sentence)
		if len(highlights) == 2 {
			break
		}
	}

	if len(events) > 0 {
		highlights = append(highlights, buildTriggerBullet(events))
	}
	if screenshotCount > 0 {
		highlights = append(highlights, fmt.Sprintf("Скриншоты по триггерам: %d шт.", screenshotCount))
	}

	if len(highlights) > 4 {
		highlights = highlights[:4]
	}

	return highlights
}

func buildTriggerSentence(events []domaintrigger.Event, screenshotCount int) string {
	mentions := uniqueTriggerMentions(events, 3)
	if len(mentions) == 0 {
		if screenshotCount > 0 {
			return fmt.Sprintf("Найдено %d триггерных совпадений, по %d из них доступны скриншоты.", len(events), screenshotCount)
		}
		return fmt.Sprintf("Найдено %d триггерных совпадений.", len(events))
	}

	message := fmt.Sprintf("Ключевые триггеры: %s.", strings.Join(mentions, ", "))
	if screenshotCount > 0 {
		message += fmt.Sprintf(" Скриншоты доступны: %d.", screenshotCount)
	}

	return message
}

func buildTriggerBullet(events []domaintrigger.Event) string {
	mentions := uniqueTriggerMentions(events, 3)
	if len(mentions) == 0 {
		return fmt.Sprintf("Найдено триггерных совпадений: %d.", len(events))
	}

	return "Триггеры: " + strings.Join(mentions, ", ") + "."
}

func uniqueTriggerMentions(events []domaintrigger.Event, limit int) []string {
	seen := make(map[string]struct{}, len(events))
	mentions := make([]string, 0, limit)
	for _, event := range events {
		parts := make([]string, 0, 2)
		if value := strings.TrimSpace(event.MatchedText); value != "" {
			parts = append(parts, value)
		}
		if value := strings.TrimSpace(event.Category); value != "" {
			parts = append(parts, value)
		}
		if len(parts) == 0 {
			continue
		}

		mention := strings.Join(parts, " / ")
		if _, ok := seen[mention]; ok {
			continue
		}
		seen[mention] = struct{}{}
		mentions = append(mentions, mention)
		if len(mentions) == limit {
			break
		}
	}

	return mentions
}

func normalizeSummaryText(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func splitSentences(value string) []string {
	if value == "" {
		return nil
	}

	normalized := strings.NewReplacer("!", ".", "?", ".", "\n", ". ").Replace(value)
	rawParts := strings.Split(normalized, ".")
	sentences := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		sentence := strings.TrimSpace(part)
		if sentence == "" {
			continue
		}
		if len(sentence) > 220 {
			sentence = strings.TrimSpace(sentence[:217]) + "..."
		}
		if !strings.HasSuffix(sentence, ".") {
			sentence += "."
		}
		sentences = append(sentences, sentence)
	}

	if len(sentences) == 0 {
		return []string{value}
	}

	return sentences
}
