package media

import (
	"path/filepath"
	"testing"
)

func TestBuildPreviewOutputRelativePath(t *testing.T) {
	t.Parallel()

	got := BuildPreviewOutputRelativePath(42, "voice sample(1).mp4", "2026-04-03")
	want := filepath.ToSlash(filepath.Join("2026-04-03", "media_42_voice_sample_1_preview.mp4"))
	if got != want {
		t.Fatalf("BuildPreviewOutputRelativePath() = %q, want %q", got, want)
	}
}

func TestBuildPreviewFFmpegArgs(t *testing.T) {
	t.Parallel()

	got := BuildPreviewFFmpegArgs("input.mov", "output.mp4")
	wantContains := []string{
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-movflags", "+faststart",
		"-c:a", "aac",
		"-vf", "scale=-2:720:force_original_aspect_ratio=decrease",
		"output.mp4",
	}

	for _, want := range wantContains {
		found := false
		for _, item := range got {
			if item == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("BuildPreviewFFmpegArgs() missing %q in %#v", want, got)
		}
	}
}
