package handlers

import (
	"fmt"
	"math"
	"time"
)

func HumanSize(sizeBytes int64) string {
	if sizeBytes < 1024 {
		return fmt.Sprintf("%d B", sizeBytes)
	}

	units := []string{"KB", "MB", "GB", "TB"}
	size := float64(sizeBytes)
	for _, unit := range units {
		size = size / 1024
		if size < 1024 {
			return fmt.Sprintf("%.1f %s", size, unit)
		}
	}

	return fmt.Sprintf("%.1f PB", size/1024)
}

func FormatTimestamp(totalSeconds float64) string {
	if totalSeconds < 0 {
		totalSeconds = 0
	}

	duration := time.Duration(math.Round(totalSeconds * float64(time.Second)))
	hours := duration / time.Hour
	duration -= hours * time.Hour
	minutes := duration / time.Minute
	duration -= minutes * time.Minute
	seconds := duration / time.Second
	duration -= seconds * time.Second
	milliseconds := duration / time.Millisecond

	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, milliseconds)
}

func FormatDateTimeUTC(value time.Time) string {
	if value.IsZero() {
		return ""
	}

	return value.UTC().Format("2006-01-02 15:04:05")
}
