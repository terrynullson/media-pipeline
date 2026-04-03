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
	tempOutputPath := buildTempScreenshotPath(fullOutputPath)

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return ports.ExtractScreenshotOutput{}, fmt.Errorf("create screenshot output dir: %w", err)
	}
	if err := os.Remove(tempOutputPath); err != nil && !os.IsNotExist(err) {
		return ports.ExtractScreenshotOutput{}, fmt.Errorf("remove stale temporary screenshot output: %w", err)
	}

	args := BuildScreenshotFFmpegArgs(in.InputPath, tempOutputPath, in.TimestampSec)
	cmd := exec.CommandContext(ctx, e.binary, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		_ = os.Remove(tempOutputPath)
		return ports.ExtractScreenshotOutput{
			ImagePath: relativeOutputPath,
			Stderr:    stderr.String(),
		}, fmt.Errorf("run ffmpeg screenshot extraction: %w", err)
	}

	file, err := os.Open(tempOutputPath)
	if err != nil {
		_ = os.Remove(tempOutputPath)
		return ports.ExtractScreenshotOutput{
			ImagePath: relativeOutputPath,
			Stderr:    stderr.String(),
		}, fmt.Errorf("open screenshot output: %w", err)
	}

	config, _, err := image.DecodeConfig(file)
	closeErr := file.Close()
	if err != nil {
		_ = os.Remove(tempOutputPath)
		return ports.ExtractScreenshotOutput{
			ImagePath: relativeOutputPath,
			Stderr:    stderr.String(),
		}, fmt.Errorf("decode screenshot dimensions: %w", err)
	}
	if closeErr != nil {
		_ = os.Remove(tempOutputPath)
		return ports.ExtractScreenshotOutput{
			ImagePath: relativeOutputPath,
			Stderr:    stderr.String(),
		}, fmt.Errorf("close screenshot output: %w", closeErr)
	}
	if err := replaceFileAtomically(tempOutputPath, fullOutputPath); err != nil {
		_ = os.Remove(tempOutputPath)
		return ports.ExtractScreenshotOutput{
			ImagePath: relativeOutputPath,
			Stderr:    stderr.String(),
		}, fmt.Errorf("promote screenshot output: %w", err)
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

func replaceFileAtomically(tempPath string, finalPath string) error {
	backupPath := finalPath + ".bak"
	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale backup file: %w", err)
	}

	if _, err := os.Stat(finalPath); err == nil {
		if err := os.Rename(finalPath, backupPath); err != nil {
			return fmt.Errorf("move existing screenshot to backup: %w", err)
		}
		if err := os.Rename(tempPath, finalPath); err != nil {
			restoreErr := os.Rename(backupPath, finalPath)
			if restoreErr != nil {
				return fmt.Errorf("replace screenshot file: %w (restore backup: %v)", err, restoreErr)
			}
			return fmt.Errorf("replace screenshot file: %w", err)
		}
		if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove screenshot backup file: %w", err)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat final screenshot output: %w", err)
	}

	if err := os.Rename(tempPath, finalPath); err != nil {
		return fmt.Errorf("move screenshot output into place: %w", err)
	}

	return nil
}

func buildTempScreenshotPath(finalPath string) string {
	ext := filepath.Ext(finalPath)
	base := finalPath[:len(finalPath)-len(ext)]
	if ext == "" {
		return finalPath + ".tmp"
	}

	return base + ".tmp" + ext
}
