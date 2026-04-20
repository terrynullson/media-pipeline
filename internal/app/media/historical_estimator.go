package mediaapp

import (
	"context"
	"fmt"
	"sort"
	"time"

	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/media"
)

type HistoricalSampleReader interface {
	ListRecentHistoricalSamples(ctx context.Context, jobTypes []job.Type, limit int) ([]job.HistoricalSample, error)
}

type HistoricalEstimate struct {
	JobType           job.Type
	Available         bool
	SampleSize        int
	EstimatedDuration time.Duration
}

type HistoricalEstimator struct {
	reader HistoricalSampleReader
	limit  int
}

func NewHistoricalEstimator(reader HistoricalSampleReader) *HistoricalEstimator {
	return &HistoricalEstimator{
		reader: reader,
		limit:  60,
	}
}

func (e *HistoricalEstimator) EstimateForMedia(ctx context.Context, mediaItem media.Media, jobType job.Type) (HistoricalEstimate, error) {
	if e == nil || e.reader == nil {
		return HistoricalEstimate{JobType: jobType}, nil
	}

	samples, err := e.reader.ListRecentHistoricalSamples(ctx, []job.Type{jobType}, e.limit)
	if err != nil {
		return HistoricalEstimate{}, fmt.Errorf("load historical samples for %s: %w", jobType, err)
	}

	targetAudioOnly := mediaItem.IsAudioOnly()
	rates := make([]float64, 0, len(samples))
	for _, sample := range samples {
		if sample.JobType != jobType || sample.IsAudioOnly != targetAudioOnly || sample.DurationMS <= 0 || sample.MediaSizeBytes <= 0 {
			continue
		}
		rates = append(rates, float64(sample.DurationMS)/1000/mediaSizeMB(sample.MediaSizeBytes))
	}
	if len(rates) == 0 {
		return HistoricalEstimate{JobType: jobType}, nil
	}

	sort.Float64s(rates)
	trimmed := trimOutliers(rates)
	avgRate := average(trimmed)
	if avgRate <= 0 {
		return HistoricalEstimate{JobType: jobType}, nil
	}

	estimatedSeconds := avgRate * mediaSizeMB(mediaItem.SizeBytes)
	estimatedDuration := time.Duration(estimatedSeconds * float64(time.Second))
	if estimatedDuration < time.Second {
		estimatedDuration = time.Second
	}

	return HistoricalEstimate{
		JobType:           jobType,
		Available:         true,
		SampleSize:        len(trimmed),
		EstimatedDuration: estimatedDuration,
	}, nil
}

func mediaSizeMB(sizeBytes int64) float64 {
	if sizeBytes <= 0 {
		return 1
	}
	value := float64(sizeBytes) / (1024 * 1024)
	if value < 1 {
		return 1
	}
	return value
}

func trimOutliers(values []float64) []float64 {
	if len(values) < 5 {
		return append([]float64(nil), values...)
	}

	start := len(values) / 5
	end := len(values) - start
	if end <= start {
		return append([]float64(nil), values...)
	}
	return append([]float64(nil), values[start:end]...)
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var total float64
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}
