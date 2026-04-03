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
	settings := transcription.NormalizeSettings(in.Settings)
	args := []string{
		t.scriptPath,
		"--audio-path", in.AudioPath,
		"--backend", string(settings.Backend),
		"--model-name", settings.ModelName,
		"--device", settings.Device,
		"--compute-type", settings.ComputeType,
		"--beam-size", fmt.Sprintf("%d", settings.BeamSize),
		"--vad-enabled", fmt.Sprintf("%t", settings.VADEnabled),
	}
	if strings.TrimSpace(settings.Language) != "" {
		args = append(args, "--language", settings.Language)
	}
	resultPath, err := createTranscriptionResultPath()
	if err != nil {
		return ports.TranscribeOutput{}, &ports.TranscriptionError{
			Cause: fmt.Errorf("prepare transcription result file: %w", err),
		}
	}
	defer cleanupTranscriptionResultPath(resultPath)
	args = append(args, "--output-path", resultPath)

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

	if err := cmd.Run(); err != nil {
		diagnostics := stderr.String()
		if stdout.Len() > 0 {
			diagnostics = combineDiagnostics(diagnostics, "stdout: "+stdout.String())
		}

		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.logger.Error("python transcription script timed out",
				slog.String("script_path", t.scriptPath),
				slog.String("stderr", diagnostics),
			)
			return ports.TranscribeOutput{}, &ports.TranscriptionError{
				Cause:       fmt.Errorf("python transcription timed out: %w", ctx.Err()),
				Diagnostics: diagnostics,
			}
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			t.logger.Error("python transcription script canceled",
				slog.String("script_path", t.scriptPath),
				slog.String("stderr", diagnostics),
			)
			return ports.TranscribeOutput{}, &ports.TranscriptionError{
				Cause:       fmt.Errorf("python transcription canceled: %w", ctx.Err()),
				Diagnostics: diagnostics,
			}
		}
		t.logger.Error("python transcription script failed",
			slog.String("script_path", t.scriptPath),
			slog.Any("error", err),
			slog.String("stderr", diagnostics),
		)
		return ports.TranscribeOutput{}, &ports.TranscriptionError{
			Cause:       fmt.Errorf("run python transcription: %w", err),
			Diagnostics: diagnostics,
		}
	}

	parsed, err := readTranscriptionResult(resultPath)
	if err != nil {
		diagnostics := combineDiagnostics(stderr.String(), "stdout: "+stdout.String())
		t.logger.Error("python transcription script returned invalid json",
			slog.String("script_path", t.scriptPath),
			slog.String("result_path", resultPath),
			slog.String("stderr", diagnostics),
		)
		return ports.TranscribeOutput{}, &ports.TranscriptionError{
			Cause:       err,
			Diagnostics: diagnostics,
		}
	}

	t.logger.Info("python transcription script completed",
		slog.String("script_path", t.scriptPath),
		slog.Int("segments", len(parsed.Segments)),
		slog.String("stderr", stderr.String()),
	)

	return parsed, nil
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
