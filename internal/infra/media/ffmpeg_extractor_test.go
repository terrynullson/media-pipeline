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

func TestFFmpegExtractor_Extract_Smoke(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available in PATH")
	}

	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "input.wav")
	if err := os.WriteFile(inputPath, wavSampleBytes(), 0o644); err != nil {
		t.Fatalf("WriteFile(input) error = %v", err)
	}

	extractor := NewFFmpegExtractor("ffmpeg")
	result, err := extractor.Extract(context.Background(), ports.ExtractAudioInput{
		MediaID:     7,
		InputPath:   inputPath,
		StoredName:  "input.wav",
		OutputDir:   filepath.Join(tempDir, "audio"),
		ProcessedAt: "2026-04-03",
	})
	if err != nil {
		t.Fatalf("Extract() error = %v, stderr = %s", err, result.Stderr)
	}

	outputPath := filepath.Join(tempDir, "audio", filepath.FromSlash(result.OutputPath))
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Stat(output) error = %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}
}

func TestValidateExtractedAudioFileRejectsHeaderOnlyWAV(t *testing.T) {
	t.Parallel()

	outputPath := filepath.Join(t.TempDir(), "empty.wav")
	if err := os.WriteFile(outputPath, make([]byte, wavHeaderSizeBytes), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err := validateExtractedAudioFile(outputPath)
	if err == nil {
		t.Fatal("validateExtractedAudioFile() error = nil, want validation error")
	}
	if err.Error() != "extract audio produced empty output" {
		t.Fatalf("validateExtractedAudioFile() error = %q, want empty output error", err)
	}
}

func wavSampleBytes() []byte {
	return []byte{
		'R', 'I', 'F', 'F',
		0x24, 0x08, 0x00, 0x00,
		'W', 'A', 'V', 'E',
		'f', 'm', 't', ' ',
		0x10, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x01, 0x00,
		0x44, 0xAC, 0x00, 0x00,
		0x88, 0x58, 0x01, 0x00,
		0x02, 0x00, 0x10, 0x00,
		'd', 'a', 't', 'a',
		0x00, 0x08, 0x00, 0x00,
	}
}
