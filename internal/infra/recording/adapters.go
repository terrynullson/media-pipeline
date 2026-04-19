package recording

import (
	"media-pipeline/internal/app/autoupload"
	"media-pipeline/internal/transport/http/handlers"
)

// AutoUploadParser adapts ParseFilename to the autoupload.RecordingMetadataParser
// signature. Wired in the composition root so the app layer never imports
// infra.
func AutoUploadParser(name string) (*autoupload.RecordingMetadata, error) {
	res, err := ParseFilename(name)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	return &autoupload.RecordingMetadata{
		SourceName:        res.SourceName,
		StartedAtUTC:      res.StartedAtUTC,
		RawRecordingLabel: res.RawRecordingLabel,
	}, nil
}

// HandlersParser adapts ParseFilename to the handlers.RecordingMetadataParser
// signature. Wired in the composition root so the transport layer's handler
// production code does not import infra.
func HandlersParser(name string) (*handlers.RecordingMetadata, error) {
	res, err := ParseFilename(name)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	return &handlers.RecordingMetadata{
		SourceName:        res.SourceName,
		StartedAtUTC:      res.StartedAtUTC,
		RawRecordingLabel: res.RawRecordingLabel,
	}, nil
}
