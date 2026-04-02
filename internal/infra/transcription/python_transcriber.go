package transcription

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"media-pipeline/internal/domain/ports"
)

type PythonTranscriber struct {
	pythonBinary string
	scriptPath   string
}

func NewPythonTranscriber(pythonBinary string, scriptPath string) *PythonTranscriber {
	return &PythonTranscriber{
		pythonBinary: pythonBinary,
		scriptPath:   scriptPath,
	}
}

func (t *PythonTranscriber) Transcribe(ctx context.Context, in ports.TranscribeInput) (ports.TranscribeOutput, error) {
	args := []string{t.scriptPath, "--audio-path", in.AudioPath}
	if strings.TrimSpace(in.Language) != "" {
		args = append(args, "--language", in.Language)
	}

	cmd := exec.CommandContext(ctx, t.pythonBinary, args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		output := ports.TranscribeOutput{Stderr: stderr.String()}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return output, fmt.Errorf("python transcription timed out: %w", ctx.Err())
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			return output, fmt.Errorf("python transcription canceled: %w", ctx.Err())
		}
		return output, fmt.Errorf("run python transcription: %w", err)
	}

	parsed, err := ParseTranscriptionOutput(stdout.Bytes())
	if err != nil {
		return ports.TranscribeOutput{Stderr: stderr.String()}, err
	}
	parsed.Stderr = stderr.String()

	return parsed, nil
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
	var decoded scriptOutput
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return ports.TranscribeOutput{}, fmt.Errorf("decode transcription json: %w", err)
	}

	fullText := strings.TrimSpace(decoded.FullText)
	if fullText == "" {
		return ports.TranscribeOutput{}, fmt.Errorf("decode transcription json: full_text is required")
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
