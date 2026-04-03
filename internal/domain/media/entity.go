package media

import (
	"strings"
	"time"
)

type Status string

const (
	StatusUploaded       Status = "uploaded"
	StatusProcessing     Status = "processing"
	StatusAudioExtracted Status = "audio_extracted"
	StatusTranscribing   Status = "transcribing"
	StatusTranscribed    Status = "transcribed"
	StatusFailed         Status = "failed"
)

type Media struct {
	ID                 int64
	OriginalName       string
	StoredName         string
	Extension          string
	MIMEType           string
	SizeBytes          int64
	StoragePath        string
	ExtractedAudioPath string
	TranscriptText     string
	Status             Status
	CreatedAtUTC       time.Time
	UpdatedAtUTC       time.Time
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
