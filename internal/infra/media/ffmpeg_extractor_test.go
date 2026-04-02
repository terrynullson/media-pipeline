package media

import (
	"reflect"
	"testing"
)

func TestBuildOutputRelativePath(t *testing.T) {
	t.Parallel()

	got := BuildOutputRelativePath(42, "voice sample(1).mp4", "2026-04-03")
	want := "2026-04-03/media_42_voice_sample_1.wav"
	if got != want {
		t.Fatalf("BuildOutputRelativePath() = %q, want %q", got, want)
	}
}

func TestBuildFFmpegArgs(t *testing.T) {
	t.Parallel()

	got := BuildFFmpegArgs("/tmp/input.mp4", "/tmp/output.wav")
	want := []string{
		"-y",
		"-i", "/tmp/input.mp4",
		"-vn",
		"-acodec", "pcm_s16le",
		"-ar", "16000",
		"-ac", "1",
		"/tmp/output.wav",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildFFmpegArgs() = %#v, want %#v", got, want)
	}
}
