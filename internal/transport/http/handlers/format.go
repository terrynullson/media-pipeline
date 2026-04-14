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

func FormatClockUTC(value time.Time) string {
	if value.IsZero() {
		return ""
	}

	return value.UTC().Format("15:04:05")
}

// FormatSRTTimestamp converts fractional seconds to SRT timestamp format HH:MM:SS,mmm.
func FormatSRTTimestamp(totalSeconds float64) string {
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

	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, milliseconds)
}

func FormatDurationRU(value time.Duration) string {
	if value < 0 {
		value = 0
	}

	rounded := value.Round(100 * time.Millisecond)
	if rounded < time.Second {
		return fmt.Sprintf("%.1f сек", float64(rounded)/float64(time.Second))
	}

	totalSeconds := int64(rounded / time.Second)
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	switch {
	case hours > 0:
		return fmt.Sprintf("%d ч %d мин %d сек", hours, minutes, seconds)
	case minutes > 0:
		return fmt.Sprintf("%d мин %d сек", minutes, seconds)
	default:
		return fmt.Sprintf("%.1f сек", float64(rounded)/float64(time.Second))
	}
}
