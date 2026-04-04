package job

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"media-pipeline/internal/domain/transcription"
)

type Type string

type Status string

const (
	TypeUpload              Type = "upload"
	TypeExtractAudio        Type = "extract_audio"
	TypePreparePreviewVideo Type = "prepare_preview_video"
	TypeTranscribe          Type = "transcribe"
	TypeAnalyzeTriggers     Type = "analyze_triggers"
	TypeExtractScreenshots  Type = "extract_screenshots"
	TypeGenerateSummary     Type = "generate_summary"

	StatusPending Status = "pending"
	StatusRunning Status = "running"
	StatusDone    Status = "done"
	StatusFailed  Status = "failed"
)

type Job struct {
	ID                   int64
	MediaID              int64
	Type                 Type
	Payload              string
	Status               Status
	Attempts             int
	ErrorMessage         string
	CreatedAtUTC         time.Time
	UpdatedAtUTC         time.Time
	StartedAtUTC         *time.Time
	FinishedAtUTC        *time.Time
	DurationMS           *int64
	ProgressPercent      *float64
	ProgressLabel        string
	ProgressIsEstimated  bool
	ProgressUpdatedAtUTC *time.Time
}

type TranscribePayload struct {
	Settings transcription.Settings `json:"settings"`
}

func EncodeTranscribePayload(payload TranscribePayload) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal transcribe payload: %w", err)
	}

	return string(body), nil
}

func DecodeTranscribePayload(raw string) (TranscribePayload, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return TranscribePayload{}, fmt.Errorf("decode transcribe payload: empty payload")
	}

	var payload TranscribePayload
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return TranscribePayload{}, fmt.Errorf("decode transcribe payload: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return TranscribePayload{}, fmt.Errorf("decode transcribe payload: unexpected trailing data")
	}
	if bytes.Equal([]byte(trimmed), []byte("null")) {
		return TranscribePayload{}, fmt.Errorf("decode transcribe payload: empty payload")
	}

	return payload, nil
}
