package mediaapp

import (
	"context"
	"testing"
	"time"

	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/media"
)

func TestHistoricalEstimator_EstimateForMediaUsesMatchingMediaClass(t *testing.T) {
	t.Parallel()

	estimator := NewHistoricalEstimator(stubHistoricalSampleReader{
		samples: []job.HistoricalSample{
			{JobType: job.TypePreparePreviewVideo, MediaSizeBytes: 100 * 1024 * 1024, DurationMS: 100 * 1000, IsAudioOnly: false},
			{JobType: job.TypePreparePreviewVideo, MediaSizeBytes: 200 * 1024 * 1024, DurationMS: 180 * 1000, IsAudioOnly: false},
			{JobType: job.TypePreparePreviewVideo, MediaSizeBytes: 50 * 1024 * 1024, DurationMS: 20 * 1000, IsAudioOnly: true},
		},
	})

	result, err := estimator.EstimateForMedia(context.Background(), media.Media{
		SizeBytes: 150 * 1024 * 1024,
		MIMEType:  "video/mp4",
		Extension: ".mp4",
	}, job.TypePreparePreviewVideo)
	if err != nil {
		t.Fatalf("EstimateForMedia() error = %v", err)
	}
	if !result.Available {
		t.Fatal("Available = false, want true")
	}
	if result.SampleSize != 2 {
		t.Fatalf("SampleSize = %d, want 2", result.SampleSize)
	}
	if result.EstimatedDuration < 2*time.Minute || result.EstimatedDuration > 3*time.Minute {
		t.Fatalf("EstimatedDuration = %v, want around 2m15s", result.EstimatedDuration)
	}
}

func TestHistoricalEstimator_EstimateForMediaReturnsUnavailableWithoutSamples(t *testing.T) {
	t.Parallel()

	estimator := NewHistoricalEstimator(stubHistoricalSampleReader{})
	result, err := estimator.EstimateForMedia(context.Background(), media.Media{
		SizeBytes: 150 * 1024 * 1024,
		MIMEType:  "video/mp4",
		Extension: ".mp4",
	}, job.TypeExtractAudio)
	if err != nil {
		t.Fatalf("EstimateForMedia() error = %v", err)
	}
	if result.Available {
		t.Fatal("Available = true, want false")
	}
}

type stubHistoricalSampleReader struct {
	samples []job.HistoricalSample
}

func (s stubHistoricalSampleReader) ListRecentHistoricalSamples(context.Context, []job.Type, int) ([]job.HistoricalSample, error) {
	return append([]job.HistoricalSample(nil), s.samples...), nil
}
