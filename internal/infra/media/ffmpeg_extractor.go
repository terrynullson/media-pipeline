package media

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"media-pipeline/internal/domain/ports"
)

const wavHeaderSizeBytes int64 = 44

type FFmpegExtractor struct {
	binary string
}

func NewFFmpegExtractor(binary string) *FFmpegExtractor {
	return &FFmpegExtractor{binary: binary}
}

func (e *FFmpegExtractor) Extract(ctx context.Context, in ports.ExtractAudioInput) (ports.ExtractAudioOutput, error) {
	relativeOutputPath := BuildOutputRelativePath(in.MediaID, in.StoredName, in.ProcessedAt)
	fullOutputPath := filepath.Join(in.OutputDir, filepath.FromSlash(relativeOutputPath))
	outputDir := filepath.Dir(fullOutputPath)

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return ports.ExtractAudioOutput{}, fmt.Errorf("create audio output dir: %w", err)
	}
	if err := os.Remove(fullOutputPath); err != nil && !os.IsNotExist(err) {
		return ports.ExtractAudioOutput{}, fmt.Errorf("remove stale audio output: %w", err)
	}

	args := BuildFFmpegArgs(in.InputPath, fullOutputPath)
	cmd := exec.CommandContext(ctx, e.binary, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		_ = os.Remove(fullOutputPath)
		return ports.ExtractAudioOutput{
			OutputPath: relativeOutputPath,
			Stderr:     stderr.String(),
		}, fmt.Errorf("run ffmpeg: %w", err)
	}

	if err := validateExtractedAudioFile(fullOutputPath); err != nil {
		_ = os.Remove(fullOutputPath)
		return ports.ExtractAudioOutput{
			OutputPath: relativeOutputPath,
			Stderr:     stderr.String(),
		}, err
	}

	return ports.ExtractAudioOutput{
		OutputPath: relativeOutputPath,
		Stderr:     stderr.String(),
	}, nil
}

func validateExtractedAudioFile(fullOutputPath string) error {
	info, err := os.Stat(fullOutputPath)
	if err != nil {
		return fmt.Errorf("inspect extracted audio output: %w", err)
	}
	if info.Size() <= wavHeaderSizeBytes {
		return fmt.Errorf("extract audio produced empty output")
	}

	return nil
}

func BuildOutputRelativePath(mediaID int64, storedName string, processedAt string) string {
	baseName := strings.TrimSuffix(storedName, filepath.Ext(storedName))
	safeBaseName := sanitizeFileComponent(baseName)
	if safeBaseName == "" {
		safeBaseName = "media"
	}

	fileName := fmt.Sprintf("media_%d_%s.wav", mediaID, safeBaseName)
	return filepath.ToSlash(filepath.Join(processedAt, fileName))
}

func BuildFFmpegArgs(inputPath string, outputPath string) []string {
	return []string{
		"-y",
		"-i", inputPath,
		"-vn",
		"-acodec", "pcm_s16le",
		"-ar", "16000",
		"-ac", "1",
		outputPath,
	}
}

func sanitizeFileComponent(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))

	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}

	return strings.Trim(builder.String(), "_")
}
