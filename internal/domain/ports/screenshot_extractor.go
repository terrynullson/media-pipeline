package ports

import "context"

type ExtractScreenshotInput struct {
	MediaID        int64
	TriggerEventID int64
	InputPath      string
	TimestampSec   float64
	OutputDir      string
	ProcessedAt    string
}

type ExtractScreenshotOutput struct {
	ImagePath string
	Width     int
	Height    int
	Stderr    string
}

type ScreenshotExtractor interface {
	Extract(ctx context.Context, in ExtractScreenshotInput) (ExtractScreenshotOutput, error)
}
