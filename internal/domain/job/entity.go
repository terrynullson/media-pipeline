package job

import "time"

type Type string

type Status string

const (
	TypeExtractAudio Type = "extract_audio"

	StatusPending Status = "pending"
	StatusRunning Status = "running"
	StatusDone    Status = "done"
	StatusFailed  Status = "failed"
)

type Job struct {
	ID           int64
	MediaID      int64
	Type         Type
	Status       Status
	Attempts     int
	ErrorMessage string
	CreatedAtUTC time.Time
	UpdatedAtUTC time.Time
}
