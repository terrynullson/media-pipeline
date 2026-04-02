package ports

import (
	"context"
	"errors"
)

type TranscriptionSegment struct {
	StartSec   float64
	EndSec     float64
	Text       string
	Confidence *float64
}

type TranscribeInput struct {
	AudioPath string
	Language  string
}

type TranscribeOutput struct {
	FullText string
	Segments []TranscriptionSegment
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
