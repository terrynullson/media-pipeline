package media

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"

	"media-pipeline/internal/domain/ports"
)

type FFmpegScreenshotExtractor struct {
	binary string
}

func NewFFmpegScreenshotExtractor(binary string) *FFmpegScreenshotExtractor {
	return &FFmpegScreenshotExtractor{binary: binary}
}

func (e *FFmpegScreenshotExtractor) Extract(
	ctx context.Context,
	in ports.ExtractScreenshotInput,
) (ports.ExtractScreenshotOutput, error) {
	relativeOutputPath := BuildScreenshotRelativePath(in.MediaID, in.TriggerEventID, in.TimestampSec, in.ProcessedAt)
	fullOutputPath := filepath.Join(in.OutputDir, filepath.FromSlash(relativeOutputPath))
	outputDir := filepath.Dir(fullOutputPath)

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return ports.ExtractScreenshotOutput{}, fmt.Errorf("create screenshot output dir: %w", err)
	}
	if err := os.Remove(fullOutputPath); err != nil && !os.IsNotExist(err) {
		return ports.ExtractScreenshotOutput{}, fmt.Errorf("remove stale screenshot output: %w", err)
	}

	args := BuildScreenshotFFmpegArgs(in.InputPath, fullOutputPath, in.TimestampSec)
	cmd := exec.CommandContext(ctx, e.binary, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		_ = os.Remove(fullOutputPath)
		return ports.ExtractScreenshotOutput{
			ImagePath: relativeOutputPath,
			Stderr:    stderr.String(),
		}, fmt.Errorf("run ffmpeg screenshot extraction: %w", err)
	}

	file, err := os.Open(fullOutputPath)
	if err != nil {
		_ = os.Remove(fullOutputPath)
		return ports.ExtractScreenshotOutput{
			ImagePath: relativeOutputPath,
			Stderr:    stderr.String(),
		}, fmt.Errorf("open screenshot output: %w", err)
	}
	defer file.Close()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		_ = os.Remove(fullOutputPath)
		return ports.ExtractScreenshotOutput{
			ImagePath: relativeOutputPath,
			Stderr:    stderr.String(),
		}, fmt.Errorf("decode screenshot dimensions: %w", err)
	}

	return ports.ExtractScreenshotOutput{
		ImagePath: relativeOutputPath,
		Width:     config.Width,
		Height:    config.Height,
		Stderr:    stderr.String(),
	}, nil
}

func BuildScreenshotRelativePath(mediaID int64, triggerEventID int64, timestampSec float64, processedAt string) string {
	fileName := fmt.Sprintf(
		"media_%d_trigger_%d_%dms.jpg",
		mediaID,
		triggerEventID,
		int64(timestampSec*1000),
	)
	return filepath.ToSlash(filepath.Join(processedAt, fileName))
}

func BuildScreenshotFFmpegArgs(inputPath string, outputPath string, timestampSec float64) []string {
	return []string{
		"-y",
		"-ss", fmt.Sprintf("%.3f", timestampSec),
		"-i", inputPath,
		"-frames:v", "1",
		"-q:v", "2",
		outputPath,
	}
}
