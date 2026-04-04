package ports

import "context"

type GeneratePreviewVideoInput struct {
	MediaID     int64
	InputPath   string
	StoredName  string
	OutputDir   string
	ProcessedAt string
}

type GeneratePreviewVideoOutput struct {
	OutputPath string
	MIMEType   string
	SizeBytes  int64
	Stderr     string
}

type PreviewVideoGenerator interface {
	Generate(ctx context.Context, in GeneratePreviewVideoInput) (GeneratePreviewVideoOutput, error)
}
