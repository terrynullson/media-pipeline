package summary

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"media-pipeline/internal/domain/ports"
)

const ProviderOllama = "ollama"

type OllamaSummarizer struct {
	baseURL string
	model   string
	client  *http.Client
	logger  *slog.Logger
	// Fallback to simple summarizer when Ollama is unavailable
	fallback *SimpleSummarizer
}

func NewOllamaSummarizer(baseURL, model string, logger *slog.Logger) *OllamaSummarizer {
	return &OllamaSummarizer{
		baseURL:  strings.TrimRight(baseURL, "/"),
		model:    model,
		client:   &http.Client{Timeout: 120 * time.Second},
		logger:   logger,
		fallback: NewSimpleSummarizer(),
	}
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

func (o *OllamaSummarizer) Generate(ctx context.Context, in ports.SummaryInput) (ports.SummaryOutput, error) {
	fullText := strings.Join(strings.Fields(strings.TrimSpace(in.Transcript.FullText)), " ")
	if fullText == "" {
		return ports.SummaryOutput{}, fmt.Errorf("summary source transcript is empty")
	}

	// Truncate to ~3000 chars to fit in context window
	if len(fullText) > 3000 {
		fullText = fullText[:3000]
	}

	// Build trigger context
	triggerInfo := ""
	if len(in.TriggerEvents) > 0 {
		mentions := make([]string, 0, 5)
		seen := make(map[string]struct{})
		for _, ev := range in.TriggerEvents {
			txt := strings.TrimSpace(ev.MatchedText)
			if txt == "" {
				continue
			}
			if _, ok := seen[txt]; ok {
				continue
			}
			seen[txt] = struct{}{}
			mentions = append(mentions, txt)
			if len(mentions) >= 5 {
				break
			}
		}
		if len(mentions) > 0 {
			triggerInfo = fmt.Sprintf("\n\nКлючевые слова-триггеры найденные в тексте: %s.", strings.Join(mentions, ", "))
		}
	}

	prompt := fmt.Sprintf(`Ты -- ассистент для создания краткого содержания медиа-контента. Напиши краткое содержание (summary) на русском языке для следующего транскрипта. Ответ должен быть 2-4 предложения, чётко и по делу, без вступлений.%s

Транскрипт:
%s

Краткое содержание:`, triggerInfo, fullText)

	result, err := o.callOllama(ctx, prompt)
	if err != nil {
		o.logger.Warn("ollama unavailable, falling back to simple summarizer", slog.Any("error", err))
		return o.fallback.Generate(ctx, in)
	}

	result = strings.TrimSpace(result)
	if result == "" {
		o.logger.Warn("ollama returned empty response, falling back")
		return o.fallback.Generate(ctx, in)
	}

	// Truncate if too long
	if len(result) > 600 {
		result = strings.TrimSpace(result[:597]) + "..."
	}

	// Build highlights from the LLM response
	sentences := splitSentences(result)
	highlights := make([]string, 0, 3)
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		highlights = append(highlights, s)
		if len(highlights) >= 3 {
			break
		}
	}

	if len(in.TriggerEvents) > 0 {
		highlights = append(highlights, fmt.Sprintf("Триггеров найдено: %d", len(in.TriggerEvents)))
	}

	return ports.SummaryOutput{
		SummaryText: result,
		Highlights:  highlights,
		Provider:    ProviderOllama + "/" + o.model,
	}, nil
}

func (o *OllamaSummarizer) callOllama(ctx context.Context, prompt string) (string, error) {
	body, err := json.Marshal(ollamaRequest{
		Model:  o.model,
		Prompt: prompt,
		Stream: false,
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result ollamaResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if result.Error != "" {
		return "", fmt.Errorf("ollama error: %s", result.Error)
	}

	return result.Response, nil
}
