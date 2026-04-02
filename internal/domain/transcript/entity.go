package transcript

import "time"

type Segment struct {
	StartSec   float64
	EndSec     float64
	Text       string
	Confidence *float64
}

type Transcript struct {
	ID           int64
	MediaID      int64
	Language     string
	FullText     string
	Segments     []Segment
	CreatedAtUTC time.Time
	UpdatedAtUTC time.Time
}
