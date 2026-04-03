package trigger

import "time"

type Screenshot struct {
	ID             int64
	MediaID        int64
	TriggerEventID int64
	TimestampSec   float64
	ImagePath      string
	Width          int
	Height         int
	CreatedAtUTC   time.Time
}
