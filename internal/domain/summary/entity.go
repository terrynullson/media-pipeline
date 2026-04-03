package summary

import "time"

type Summary struct {
	ID           int64
	MediaID      int64
	SummaryText  string
	Highlights   []string
	Provider     string
	CreatedAtUTC time.Time
	UpdatedAtUTC time.Time
}
