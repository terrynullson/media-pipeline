package media

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

type Status string

const (
	StatusQueued         Status = "queued"
	StatusUploaded       Status = "uploaded"
	StatusProcessing     Status = "processing"
	StatusAudioExtracted Status = "audio_extracted"
	StatusTranscribing   Status = "transcribing"
	StatusTranscribed    Status = "transcribed"
	StatusFailed         Status = "failed"
)

type Media struct {
	ID                       int64
	OriginalName             string
	StoredName               string
	Extension                string
	MIMEType                 string
	SizeBytes                int64
	StoragePath              string
	ExtractedAudioPath       string
	PreviewVideoPath         string
	PreviewVideoSizeBytes    int64
	PreviewVideoMIMEType     string
	PreviewVideoCreatedAtUTC *time.Time
	TranscriptText           string
	RuntimeSnapshotJSON      string
	Status                   Status
	CreatedAtUTC             time.Time
	UpdatedAtUTC             time.Time
}

type RuntimeSnapshot struct {
	CapturedAtUTC         time.Time `json:"captured_at_utc"`
	RequestIP             string    `json:"request_ip,omitempty"`
	UserAgent             string    `json:"user_agent,omitempty"`
	AcceptLanguage        string    `json:"accept_language,omitempty"`
	ClientLanguage        string    `json:"client_language,omitempty"`
	ClientPlatform        string    `json:"client_platform,omitempty"`
	ClientHintPlatform    string    `json:"client_hint_platform,omitempty"`
	ClientHintMobile      string    `json:"client_hint_mobile,omitempty"`
	ClientHintArch        string    `json:"client_hint_arch,omitempty"`
	ClientHintBitness     string    `json:"client_hint_bitness,omitempty"`
	HardwareConcurrency   *int      `json:"hardware_concurrency,omitempty"`
	DeviceMemoryGB        *float64  `json:"device_memory_gb,omitempty"`
	TimezoneOffsetMinutes *int      `json:"timezone_offset_minutes,omitempty"`
}

func (m Media) IsAudioOnly() bool {
	mimeType := strings.ToLower(strings.TrimSpace(m.MIMEType))
	if strings.HasPrefix(mimeType, "audio/") {
		return true
	}
	if strings.HasPrefix(mimeType, "video/") {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(m.Extension)) {
	case ".mp3", ".wav", ".m4a", ".aac", ".flac":
		return true
	default:
		return false
	}
}

func EncodeRuntimeSnapshot(snapshot RuntimeSnapshot) (string, error) {
	body, err := json.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("marshal runtime snapshot: %w", err)
	}

	return string(body), nil
}

func DecodeRuntimeSnapshot(raw string) (RuntimeSnapshot, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return RuntimeSnapshot{}, fmt.Errorf("decode runtime snapshot: empty payload")
	}

	var snapshot RuntimeSnapshot
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot); err != nil {
		return RuntimeSnapshot{}, fmt.Errorf("decode runtime snapshot: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return RuntimeSnapshot{}, fmt.Errorf("decode runtime snapshot: unexpected trailing data")
	}
	if bytes.Equal([]byte(trimmed), []byte("null")) {
		return RuntimeSnapshot{}, fmt.Errorf("decode runtime snapshot: empty payload")
	}

	return snapshot, nil
}
