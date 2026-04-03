package ports

import (
	"context"
	"errors"

	"media-pipeline/internal/domain/transcription"
)

type TranscriptionSegment struct {
	StartSec   float64
	EndSec     float64
	Text       string
	Confidence *float64
}

type TranscribeInput struct {
	AudioPath string
	Settings  transcription.Settings
	Progress  func(TranscriptionProgress)
}

type TranscribeOutput struct {
	FullText string
	Segments []TranscriptionSegment
}

type TranscriptionProgress struct {
	ProcessedSec float64
	TotalSec     float64
	Percent      float64
	IsEstimate   bool
}

type Transcriber interface {
	Transcribe(ctx context.Context, in TranscribeInput) (TranscribeOutput, error)
}

type TranscriptionError struct {
	Cause       error
	Diagnostics string
}

func (e *TranscriptionError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return "transcription failed"
	}

	return e.Cause.Error()
}

func (e *TranscriptionError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Cause
}

func AsTranscriptionError(err error) (*TranscriptionError, bool) {
	var target *TranscriptionError
	if errors.As(err, &target) {
		return target, true
	}

	return nil, false
}
