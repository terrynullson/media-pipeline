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

type FFmpegPreviewGenerator struct {
	binary string
}

func NewFFmpegPreviewGenerator(binary string) *FFmpegPreviewGenerator {
	return &FFmpegPreviewGenerator{binary: binary}
}

func (g *FFmpegPreviewGenerator) Generate(
	ctx context.Context,
	in ports.GeneratePreviewVideoInput,
) (ports.GeneratePreviewVideoOutput, error) {
	relativeOutputPath := BuildPreviewOutputRelativePath(in.MediaID, in.StoredName, in.ProcessedAt)
	fullOutputPath := filepath.Join(in.OutputDir, filepath.FromSlash(relativeOutputPath))
	outputDir := filepath.Dir(fullOutputPath)
	tempOutputPath := buildTempPreviewPath(fullOutputPath)

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return ports.GeneratePreviewVideoOutput{}, fmt.Errorf("create preview output dir: %w", err)
	}
	if err := os.Remove(tempOutputPath); err != nil && !os.IsNotExist(err) {
		return ports.GeneratePreviewVideoOutput{}, fmt.Errorf("remove stale temporary preview output: %w", err)
	}

	args := BuildPreviewFFmpegArgs(in.InputPath, tempOutputPath)
	cmd := exec.CommandContext(ctx, g.binary, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		_ = os.Remove(tempOutputPath)
		return ports.GeneratePreviewVideoOutput{
			OutputPath: relativeOutputPath,
			MIMEType:   "video/mp4",
			Stderr:     stderr.String(),
		}, fmt.Errorf("run ffmpeg preview generation: %w", err)
	}

	info, err := os.Stat(tempOutputPath)
	if err != nil {
		_ = os.Remove(tempOutputPath)
		return ports.GeneratePreviewVideoOutput{
			OutputPath: relativeOutputPath,
			MIMEType:   "video/mp4",
			Stderr:     stderr.String(),
		}, fmt.Errorf("stat preview output: %w", err)
	}

	if err := replaceFileAtomically(tempOutputPath, fullOutputPath); err != nil {
		_ = os.Remove(tempOutputPath)
		return ports.GeneratePreviewVideoOutput{
			OutputPath: relativeOutputPath,
			MIMEType:   "video/mp4",
			Stderr:     stderr.String(),
		}, fmt.Errorf("promote preview output: %w", err)
	}

	return ports.GeneratePreviewVideoOutput{
		OutputPath: relativeOutputPath,
		MIMEType:   "video/mp4",
		SizeBytes:  info.Size(),
		Stderr:     stderr.String(),
	}, nil
}

func BuildPreviewOutputRelativePath(mediaID int64, storedName string, processedAt string) string {
	baseName := strings.TrimSuffix(storedName, filepath.Ext(storedName))
	safeBaseName := sanitizeFileComponent(baseName)
	if safeBaseName == "" {
		safeBaseName = "media"
	}

	fileName := fmt.Sprintf("media_%d_%s_preview.mp4", mediaID, safeBaseName)
	return filepath.ToSlash(filepath.Join(processedAt, fileName))
}

func BuildPreviewFFmpegArgs(inputPath string, outputPath string) []string {
	return []string{
		"-y",
		"-i", inputPath,
		"-map", "0:v:0",
		"-map", "0:a:0?",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "23",
		"-vf", "scale=-2:720:force_original_aspect_ratio=decrease",
		"-pix_fmt", "yuv420p",
		"-movflags", "+faststart",
		"-profile:v", "high",
		"-level", "4.1",
		"-c:a", "aac",
		"-b:a", "128k",
		outputPath,
	}
}

func buildTempPreviewPath(finalPath string) string {
	ext := filepath.Ext(finalPath)
	base := finalPath[:len(finalPath)-len(ext)]
	if ext == "" {
		return finalPath + ".tmp"
	}

	return base + ".tmp" + ext
}
