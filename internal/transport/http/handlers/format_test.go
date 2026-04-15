package handlers

import (
	"testing"
	"time"

	"media-pipeline/internal/domain/transcript"
)

func TestHumanSize(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		input int64
		want  string
	}{
		{name: "bytes", input: 999, want: "999 B"},
		{name: "kilobytes", input: 1536, want: "1.5 KB"},
		{name: "megabytes", input: 5 * 1024 * 1024, want: "5.0 MB"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := HumanSize(tc.input); got != tc.want {
				t.Fatalf("HumanSize(%d) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestFormatTimestamp(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		input float64
		want  string
	}{
		{name: "zero", input: 0, want: "00:00:00.000"},
		{name: "seconds", input: 61.25, want: "00:01:01.250"},
		{name: "hours", input: 3661.789, want: "01:01:01.789"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := FormatTimestamp(tc.input); got != tc.want {
				t.Fatalf("FormatTimestamp(%f) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestFormatDateTimeUTC(t *testing.T) {
	t.Parallel()

	// Input: 2026-04-03 09:34:56 UTC → expected MSK output: 03.04.2026 12:34:56
	value := time.Date(2026, 4, 3, 9, 34, 56, 0, time.UTC)
	if got := FormatDateTimeUTC(value); got != "03.04.2026 12:34:56" {
		t.Fatalf("FormatDateTimeUTC() = %q, want %q", got, "03.04.2026 12:34:56")
	}
}

func TestFormatClockUTC(t *testing.T) {
	t.Parallel()

	// Input: 2026-04-14 21:00:00 UTC → expected MSK output: 15.04.2026 00:00:00
	value := time.Date(2026, 4, 14, 21, 0, 0, 0, time.UTC)
	if got := FormatClockUTC(value); got != "15.04.2026 00:00:00" {
		t.Fatalf("FormatClockUTC() = %q, want %q", got, "15.04.2026 00:00:00")
	}
}

func TestFormatSRTTimestamp(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		input float64
		want  string
	}{
		{name: "zero", input: 0, want: "00:00:00,000"},
		{name: "seconds_millis", input: 1.2, want: "00:00:01,200"},
		{name: "multi_second", input: 3.5, want: "00:00:03,500"},
		{name: "minutes", input: 63.8, want: "00:01:03,800"},
		{name: "hours", input: 3661.0, want: "01:01:01,000"},
		{name: "negative_clamped", input: -1, want: "00:00:00,000"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := FormatSRTTimestamp(tc.input); got != tc.want {
				t.Fatalf("FormatSRTTimestamp(%f) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestFormatSRT(t *testing.T) {
	t.Parallel()

	segments := []transcript.Segment{
		{StartSec: 1.2, EndSec: 3.5, Text: "Привет, мир."},
		{StartSec: 3.8, EndSec: 6.1, Text: "Как дела?"},
		{StartSec: 7.0, EndSec: 7.0, Text: "   "}, // blank — should be skipped
	}

	got := formatSRT(segments)
	want := "1\n00:00:01,200 --> 00:00:03,500\nПривет, мир.\n\n2\n00:00:03,800 --> 00:00:06,100\nКак дела?\n\n"
	if got != want {
		t.Fatalf("formatSRT() =\n%q\nwant\n%q", got, want)
	}
}

func TestFormatSRTEmpty(t *testing.T) {
	t.Parallel()

	if got := formatSRT(nil); got != "" {
		t.Fatalf("formatSRT(nil) = %q, want empty string", got)
	}
}

func TestFormatDurationRU(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		input time.Duration
		want  string
	}{
		{name: "sub-second", input: 450 * time.Millisecond, want: "0.5 сек"},
		{name: "seconds", input: 5600 * time.Millisecond, want: "5.6 сек"},
		{name: "minutes", input: 2*time.Minute + 14*time.Second, want: "2 мин 14 сек"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := FormatDurationRU(tc.input); got != tc.want {
				t.Fatalf("FormatDurationRU(%s) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
