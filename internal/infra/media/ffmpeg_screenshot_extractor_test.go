package media

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"media-pipeline/internal/domain/ports"
)

func TestBuildScreenshotRelativePath(t *testing.T) {
	t.Parallel()

	got := BuildScreenshotRelativePath(42, 7, 3.25, "2026-04-03")
	want := "2026-04-03/media_42_trigger_7_3250ms.jpg"
	if got != want {
		t.Fatalf("BuildScreenshotRelativePath() = %q, want %q", got, want)
	}
}

func TestBuildScreenshotFFmpegArgs(t *testing.T) {
	t.Parallel()

	got := BuildScreenshotFFmpegArgs("/tmp/input.mp4", "/tmp/output.jpg", 3.25)
	want := []string{
		"-y",
		"-ss", "3.250",
		"-i", "/tmp/input.mp4",
		"-frames:v", "1",
		"-q:v", "2",
		"/tmp/output.jpg",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildScreenshotFFmpegArgs() = %#v, want %#v", got, want)
	}
}

func TestReplaceFileAtomically_ReplacesExistingFile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	finalPath := filepath.Join(tempDir, "frame.jpg")
	tempPath := finalPath + ".tmp"

	if err := os.WriteFile(finalPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("WriteFile(finalPath) error = %v", err)
	}
	if err := os.WriteFile(tempPath, []byte("new"), 0o644); err != nil {
		t.Fatalf("WriteFile(tempPath) error = %v", err)
	}

	if err := replaceFileAtomically(tempPath, finalPath); err != nil {
		t.Fatalf("replaceFileAtomically() error = %v", err)
	}

	content, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("ReadFile(finalPath) error = %v", err)
	}
	if string(content) != "new" {
		t.Fatalf("final file content = %q, want %q", string(content), "new")
	}
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("temp file still exists, stat err = %v", err)
	}
	if _, err := os.Stat(finalPath + ".bak"); !os.IsNotExist(err) {
		t.Fatalf("backup file still exists, stat err = %v", err)
	}
}

func TestBuildTempScreenshotPath_PreservesImageExtension(t *testing.T) {
	t.Parallel()

	got := buildTempScreenshotPath(filepath.Join("shots", "frame.jpg"))
	want := filepath.Join("shots", "frame.tmp.jpg")
	if got != want {
		t.Fatalf("buildTempScreenshotPath() = %q, want %q", got, want)
	}
}

func TestFFmpegScreenshotExtractor_Extract_Smoke(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available in PATH")
	}

	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "input.mp4")
	generateVideo := exec.Command(
		"ffmpeg",
		"-y",
		"-f", "lavfi",
		"-i", "color=c=black:s=320x180:d=1",
		"-pix_fmt", "yuv420p",
		inputPath,
	)
	if output, err := generateVideo.CombinedOutput(); err != nil {
		t.Fatalf("generate test video error = %v, output = %s", err, string(output))
	}

	extractor := NewFFmpegScreenshotExtractor("ffmpeg")
	result, err := extractor.Extract(context.Background(), ports.ExtractScreenshotInput{
		MediaID:        1,
		TriggerEventID: 2,
		InputPath:      inputPath,
		OutputDir:      filepath.Join(tempDir, "screenshots"),
		TimestampSec:   0,
		ProcessedAt:    "2026-04-03",
	})
	if err != nil {
		t.Fatalf("Extract() error = %v, stderr = %s", err, result.Stderr)
	}

	outputPath := filepath.Join(tempDir, "screenshots", filepath.FromSlash(result.ImagePath))
	info, statErr := os.Stat(outputPath)
	if statErr != nil {
		t.Fatalf("Stat(output) error = %v", statErr)
	}
	if info.Size() == 0 {
		t.Fatal("screenshot file is empty")
	}
	if result.Width <= 0 || result.Height <= 0 {
		t.Fatalf("dimensions = %dx%d, want positive", result.Width, result.Height)
	}
}
