package ports

import "context"

type ExtractAudioInput struct {
	MediaID     int64
	InputPath   string
	StoredName  string
	OutputDir   string
	ProcessedAt string
}

type ExtractAudioOutput struct {
	OutputPath string
	Stderr     string
}

type AudioExtractor interface {
	Extract(ctx context.Context, in ExtractAudioInput) (ExtractAudioOutput, error)
}
