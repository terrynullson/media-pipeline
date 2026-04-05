package transcription

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"media-pipeline/internal/domain/ports"
	"media-pipeline/internal/domain/transcription"
)

const maxScriptOutputBytes = 64 * 1024

type PythonTranscriber struct {
	pythonBinary string
	scriptPath   string
	logger       *slog.Logger
}

func NewPythonTranscriber(pythonBinary string, scriptPath string, logger *slog.Logger) *PythonTranscriber {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &PythonTranscriber{
		pythonBinary: pythonBinary,
		scriptPath:   scriptPath,
		logger:       logger,
	}
}

func (t *PythonTranscriber) Transcribe(ctx context.Context, in ports.TranscribeInput) (ports.TranscribeOutput, error) {
	startedAt := time.Now()
	settings := transcription.NormalizeSettings(in.Settings)
	resultPath, err := createTranscriptionResultPath()
	if err != nil {
		return ports.TranscribeOutput{}, &ports.TranscriptionError{
			Cause: fmt.Errorf("prepare transcription result file: %w", err),
		}
	}
	defer cleanupTranscriptionResultPath(resultPath)
	progressPath, err := createTranscriptionResultPath()
	if err != nil {
		return ports.TranscribeOutput{}, &ports.TranscriptionError{
			Cause: fmt.Errorf("prepare transcription progress file: %w", err),
		}
	}
	defer cleanupTranscriptionResultPath(progressPath)
	current := settings
	for {
		parsed, runErr := t.runTranscriptionAttempt(ctx, in, current, resultPath, progressPath, startedAt)
		if runErr == nil {
			return parsed, nil
		}
		fallback, ok := fallbackTranscriptionSettings(current, runErr)
		if !ok {
			return ports.TranscribeOutput{}, runErr
		}
		t.logger.Warn("python transcription retrying with fallback compute type",
			slog.String("audio_path", in.AudioPath),
			slog.String("device", current.Device),
			slog.String("compute_type", current.ComputeType),
			slog.String("fallback_compute_type", fallback.ComputeType),
			slog.String("fallback_device", fallback.Device),
			slog.String("fallback_model_name", fallback.ModelName),
		)
		current = fallback
	}
}

func (t *PythonTranscriber) runTranscriptionAttempt(
	ctx context.Context,
	in ports.TranscribeInput,
	settings transcription.Settings,
	resultPath string,
	progressPath string,
	startedAt time.Time,
) (ports.TranscribeOutput, error) {
	args := buildTranscriptionArgs(t.scriptPath, in.AudioPath, settings, resultPath, progressPath, in.Progress != nil)
	cmd := exec.CommandContext(ctx, t.pythonBinary, args...)

	stdout := newLimitedBuffer(maxScriptOutputBytes)
	stderr := newLimitedBuffer(maxScriptOutputBytes)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	t.logger.Info("python transcription script started",
		slog.String("script_path", t.scriptPath),
		slog.String("audio_path", in.AudioPath),
		slog.String("backend", string(settings.Backend)),
		slog.String("model_name", settings.ModelName),
		slog.String("device", settings.Device),
		slog.String("compute_type", settings.ComputeType),
		slog.String("language", settings.Language),
		slog.Int("beam_size", settings.BeamSize),
		slog.Bool("vad_enabled", settings.VADEnabled),
	)

	stopProgress := make(chan struct{})
	progressDone := make(chan struct{})
	if in.Progress != nil {
		go func() {
			defer close(progressDone)
			pollTranscriptionProgress(ctx, progressPath, in.Progress, stopProgress)
		}()
	} else {
		close(progressDone)
	}

	if err := cmd.Run(); err != nil {
		close(stopProgress)
		<-progressDone
		diagnostics := stderr.String()
		if stdout.Len() > 0 {
			diagnostics = combineDiagnostics(diagnostics, "stdout: "+stdout.String())
		}
		diagnosticExcerpt := summarizeDiagnostics(diagnostics)

		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.logger.Error("python transcription script timed out",
				slog.String("script_path", t.scriptPath),
				slog.String("diagnostics_excerpt", diagnosticExcerpt),
				slog.Duration("duration", time.Since(startedAt)),
			)
			return ports.TranscribeOutput{}, &ports.TranscriptionError{
				Cause:       fmt.Errorf("python transcription timed out: %w", ctx.Err()),
				Diagnostics: diagnostics,
			}
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			t.logger.Error("python transcription script canceled",
				slog.String("script_path", t.scriptPath),
				slog.String("diagnostics_excerpt", diagnosticExcerpt),
				slog.Duration("duration", time.Since(startedAt)),
			)
			return ports.TranscribeOutput{}, &ports.TranscriptionError{
				Cause:       fmt.Errorf("python transcription canceled: %w", ctx.Err()),
				Diagnostics: diagnostics,
			}
		}
		t.logger.Error("python transcription script failed",
			slog.String("script_path", t.scriptPath),
			slog.Any("error", err),
			slog.String("diagnostics_excerpt", diagnosticExcerpt),
			slog.Duration("duration", time.Since(startedAt)),
		)
		return ports.TranscribeOutput{}, &ports.TranscriptionError{
			Cause:       fmt.Errorf("run python transcription: %w", err),
			Diagnostics: diagnostics,
		}
	}

	close(stopProgress)
	<-progressDone

	parsed, err := readTranscriptionResult(resultPath)
	if err != nil {
		diagnostics := combineDiagnostics(stderr.String(), "stdout: "+stdout.String())
		t.logger.Error("python transcription script returned invalid json",
			slog.String("script_path", t.scriptPath),
			slog.String("result_path", resultPath),
			slog.String("diagnostics_excerpt", summarizeDiagnostics(diagnostics)),
			slog.Duration("duration", time.Since(startedAt)),
		)
		return ports.TranscribeOutput{}, &ports.TranscriptionError{
			Cause:       err,
			Diagnostics: diagnostics,
		}
	}

	t.logger.Info("python transcription script completed",
		slog.String("script_path", t.scriptPath),
		slog.Int("segments", len(parsed.Segments)),
		slog.String("stderr_excerpt", summarizeDiagnostics(stderr.String())),
		slog.Duration("duration", time.Since(startedAt)),
	)

	return parsed, nil
}

func buildTranscriptionArgs(
	scriptPath string,
	audioPath string,
	settings transcription.Settings,
	resultPath string,
	progressPath string,
	withProgress bool,
) []string {
	args := []string{
		scriptPath,
		"--audio-path", audioPath,
		"--backend", string(settings.Backend),
		"--model-name", settings.ModelName,
		"--device", settings.Device,
		"--compute-type", settings.ComputeType,
		"--beam-size", fmt.Sprintf("%d", settings.BeamSize),
		"--vad-enabled", fmt.Sprintf("%t", settings.VADEnabled),
		"--output-path", resultPath,
	}
	if strings.TrimSpace(settings.Language) != "" {
		args = append(args, "--language", settings.Language)
	}
	if withProgress {
		args = append(args, "--progress-path", progressPath)
	}

	return args
}

func fallbackTranscriptionSettings(
	settings transcription.Settings,
	err error,
) (transcription.Settings, bool) {
	transcriptionErr, ok := ports.AsTranscriptionError(err)
	if !ok {
		return transcription.Settings{}, false
	}

	diagnostics := strings.ToLower(strings.TrimSpace(transcriptionErr.Diagnostics))
	if settings.Device != "cuda" {
		return transcription.Settings{}, false
	}

	fallback := settings
	switch {
	case settings.ComputeType == "float16" && strings.Contains(diagnostics, "requested float16 compute type"):
		fallback.ComputeType = "int8_float16"
		return fallback, true
	case settings.ComputeType == "int8_float16" && strings.Contains(diagnostics, "requested int8_float16 compute type"):
		fallback.ComputeType = "int8_float32"
		return fallback, true
	case shouldFallbackToSafeCPU(settings, transcriptionErr):
		fallback.Device = "cpu"
		fallback.ComputeType = "int8"
		if settings.ModelName != "tiny" {
			fallback.ModelName = "tiny"
		}
		return fallback, true
	default:
		return transcription.Settings{}, false
	}
}

func shouldFallbackToSafeCPU(settings transcription.Settings, err *ports.TranscriptionError) bool {
	if settings.Device != "cuda" || err == nil {
		return false
	}

	diagnostics := strings.ToLower(strings.TrimSpace(err.Diagnostics))
	causeText := strings.ToLower(strings.TrimSpace(err.Error()))

	if diagnostics != "" {
		for _, marker := range []string{
			"requested int8_float32 compute type",
			"segmentation fault",
			"access violation",
			"stack buffer overrun",
			"illegal instruction",
		} {
			if strings.Contains(diagnostics, marker) {
				return true
			}
		}
	}

	for _, marker := range []string{
		"exit status 3221226505",
		"exit status -1073740791",
		"exit status 3221225477",
		"exit status -1073741819",
	} {
		if strings.Contains(causeText, marker) {
			return true
		}
	}

	return diagnostics == ""
}

type transcriptionProgressFile struct {
	ProcessedSec float64 `json:"processed_sec"`
	TotalSec     float64 `json:"total_sec"`
	Percent      float64 `json:"percent"`
	IsEstimate   bool    `json:"is_estimate"`
}

func createTranscriptionResultPath() (string, error) {
	file, err := os.CreateTemp("", "media-pipeline-transcription-*.json")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close temp result file: %w", err)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("remove temp result placeholder: %w", err)
	}

	return path, nil
}

func cleanupTranscriptionResultPath(path string) {
	if strings.TrimSpace(path) == "" {
		return
	}

	_ = os.Remove(path)
	_ = os.Remove(path + ".tmp")
}

func readTranscriptionResult(path string) (ports.TranscribeOutput, error) {
	if strings.TrimSpace(path) == "" {
		return ports.TranscribeOutput{}, fmt.Errorf("decode transcription json: result path is empty")
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ports.TranscribeOutput{}, fmt.Errorf("decode transcription json: result file not found")
		}
		return ports.TranscribeOutput{}, fmt.Errorf("read transcription result file %s: %w", filepath.Base(path), err)
	}

	return ParseTranscriptionOutput(payload)
}

type scriptOutput struct {
	FullText string          `json:"full_text"`
	Segments []scriptSegment `json:"segments"`
}

type scriptSegment struct {
	StartSec   float64  `json:"start_sec"`
	EndSec     float64  `json:"end_sec"`
	Text       string   `json:"text"`
	Confidence *float64 `json:"confidence,omitempty"`
}

func ParseTranscriptionOutput(payload []byte) (ports.TranscribeOutput, error) {
	trimmedPayload := bytes.TrimSpace(payload)
	if len(trimmedPayload) == 0 {
		return ports.TranscribeOutput{}, fmt.Errorf("decode transcription json: empty output")
	}

	var decoded scriptOutput
	decoder := json.NewDecoder(bytes.NewReader(trimmedPayload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return ports.TranscribeOutput{}, fmt.Errorf("decode transcription json: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ports.TranscribeOutput{}, fmt.Errorf("decode transcription json: unexpected trailing data")
	}

	fullText := strings.TrimSpace(decoded.FullText)
	if fullText == "" {
		return ports.TranscribeOutput{}, fmt.Errorf("decode transcription json: full_text is required")
	}
	if len(decoded.Segments) == 0 {
		return ports.TranscribeOutput{}, fmt.Errorf("decode transcription json: at least one segment is required")
	}

	segments := make([]ports.TranscriptionSegment, 0, len(decoded.Segments))
	for index, segment := range decoded.Segments {
		if err := validateSegment(index, segment); err != nil {
			return ports.TranscribeOutput{}, err
		}
		segments = append(segments, ports.TranscriptionSegment{
			StartSec:   segment.StartSec,
			EndSec:     segment.EndSec,
			Text:       strings.TrimSpace(segment.Text),
			Confidence: segment.Confidence,
		})
	}

	return ports.TranscribeOutput{
		FullText: fullText,
		Segments: segments,
	}, nil
}

func validateSegment(index int, segment scriptSegment) error {
	if segment.StartSec < 0 {
		return fmt.Errorf("decode transcription json: segments[%d].start_sec must be >= 0", index)
	}
	if segment.EndSec < segment.StartSec {
		return fmt.Errorf("decode transcription json: segments[%d].end_sec must be >= start_sec", index)
	}
	if strings.TrimSpace(segment.Text) == "" {
		return fmt.Errorf("decode transcription json: segments[%d].text is required", index)
	}

	return nil
}

type limitedBuffer struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func newLimitedBuffer(limit int) limitedBuffer {
	return limitedBuffer{limit: limit}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		b.truncated = true
		return len(p), nil
	}
	remaining := b.limit - b.buffer.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.buffer.Write(p[:remaining])
		b.truncated = true
		return len(p), nil
	}

	return b.buffer.Write(p)
}

func (b *limitedBuffer) Bytes() []byte {
	return []byte(b.String())
}

func (b *limitedBuffer) Len() int {
	return len(b.String())
}

func (b *limitedBuffer) String() string {
	if !b.truncated {
		return b.buffer.String()
	}

	return b.buffer.String() + "\n[truncated]"
}

func combineDiagnostics(parts ...string) string {
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		values = append(values, part)
	}

	return strings.Join(values, "\n")
}

func summarizeDiagnostics(raw string) string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		line := strings.Join(strings.Fields(strings.TrimSpace(lines[index])), " ")
		if line == "" {
			continue
		}
		return truncateText(line, 400)
	}

	return ""
}

func truncateText(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}

	return value[:limit]
}

func pollTranscriptionProgress(
	ctx context.Context,
	progressPath string,
	onProgress func(ports.TranscriptionProgress),
	stop <-chan struct{},
) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	lastPercent := -1.0
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			reportProgressFile(progressPath, onProgress, &lastPercent)
			return
		case <-ticker.C:
			reportProgressFile(progressPath, onProgress, &lastPercent)
		}
	}
}

func reportProgressFile(progressPath string, onProgress func(ports.TranscriptionProgress), lastPercent *float64) {
	progress, ok := readTranscriptionProgress(progressPath)
	if !ok {
		return
	}
	if lastPercent != nil && progress.Percent == *lastPercent {
		return
	}
	if lastPercent != nil {
		*lastPercent = progress.Percent
	}

	onProgress(ports.TranscriptionProgress{
		ProcessedSec: progress.ProcessedSec,
		TotalSec:     progress.TotalSec,
		Percent:      progress.Percent,
		IsEstimate:   progress.IsEstimate,
	})
}

func readTranscriptionProgress(progressPath string) (transcriptionProgressFile, bool) {
	payload, err := os.ReadFile(progressPath)
	if err != nil {
		return transcriptionProgressFile{}, false
	}

	var progress transcriptionProgressFile
	if err := json.Unmarshal(payload, &progress); err != nil {
		return transcriptionProgressFile{}, false
	}
	if progress.Percent < 0 {
		progress.Percent = 0
	}
	if progress.Percent > 100 {
		progress.Percent = 100
	}

	return progress, true
}
