package ports

import "context"

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
	Stderr   string
}

type Transcriber interface {
	Transcribe(ctx context.Context, in TranscribeInput) (TranscribeOutput, error)
}
