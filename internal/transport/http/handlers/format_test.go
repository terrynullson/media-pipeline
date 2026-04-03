package handlers

import (
	"testing"
	"time"
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

	value := time.Date(2026, 4, 3, 12, 34, 56, 0, time.FixedZone("MSK", 3*60*60))
	if got := FormatDateTimeUTC(value); got != "2026-04-03 09:34:56" {
		t.Fatalf("FormatDateTimeUTC() = %q, want %q", got, "2026-04-03 09:34:56")
	}
}
