package media

import "time"

type Status string

const (
	StatusUploaded       Status = "uploaded"
	StatusProcessing     Status = "processing"
	StatusAudioExtracted Status = "audio_extracted"
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
	Status             Status
	CreatedAtUTC       time.Time
	UpdatedAtUTC       time.Time
}
