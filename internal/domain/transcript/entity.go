package transcript

import "time"

// Segment is one faster-whisper segment. StartSec/EndSec are relative to the
// recording origin (always available). StartedAtUTC/EndedAtUTC are absolute
// wall-clock times derived from media.recording_started_at when the source
// recording's timecode is known; otherwise nil.
type Segment struct {
	StartSec     float64
	EndSec       float64
	Text         string
	Confidence   *float64
	StartedAtUTC *time.Time
	EndedAtUTC   *time.Time
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

// Window is a pre-aggregated analytic bucket — e.g. all segment text spoken in
// a 10-second slice of broadcast time. Used by exports and analytic UIs so
// they don't have to re-aggregate raw segments at query time.
type Window struct {
	ID              int64
	MediaID         int64
	TranscriptID    int64
	WindowSizeSec   int
	WindowStartedAt time.Time
	WindowEndedAt   time.Time
	Text            string
	SegmentCount    int
	CreatedAtUTC    time.Time
}
