package media

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseFFmpegTimestamp(t *testing.T) {
	t.Parallel()

	got, err := parseFFmpegTimestamp("00:01:30.500000")
	if err != nil {
		t.Fatalf("parseFFmpegTimestamp() error = %v", err)
	}
	if got != 90.5 {
		t.Fatalf("parseFFmpegTimestamp() = %v, want 90.5", got)
	}
}

func TestConsumeFFmpegStderrExtractsDuration(t *testing.T) {
	t.Parallel()

	input := strings.NewReader("ffmpeg version ...\nDuration: 00:02:03.25, start: 0.000000, bitrate: 128 kb/s\n")
	var parsed float64
	var stderr bytes.Buffer

	if err := consumeFFmpegStderr(input, &stderr, func(durationSec float64) {
		parsed = durationSec
	}); err != nil {
		t.Fatalf("consumeFFmpegStderr() error = %v", err)
	}
	if parsed != 123.25 {
		t.Fatalf("parsed duration = %v, want 123.25", parsed)
	}
	if !strings.Contains(stderr.String(), "Duration: 00:02:03.25") {
		t.Fatalf("stderr buffer = %q, want original line", stderr.String())
	}
}
