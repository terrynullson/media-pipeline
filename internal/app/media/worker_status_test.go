package mediaapp

import (
	"context"
	"testing"
	"time"

	"media-pipeline/internal/domain/job"
)

type stubWorkerJobReader struct {
	running []job.Job
	pending []job.Job
}

func (s stubWorkerJobReader) ListAllByStatus(_ context.Context, status job.Status) ([]job.Job, error) {
	switch status {
	case job.StatusRunning:
		return append([]job.Job(nil), s.running...), nil
	case job.StatusPending:
		return append([]job.Job(nil), s.pending...), nil
	default:
		return nil, nil
	}
}

func TestWorkerStatusLongRunningJobStaysAliveWithinGrace(t *testing.T) {
	now := time.Now().UTC()
	startedAt := now.Add(-8 * time.Minute)
	running := job.Job{
		ID:           11,
		MediaID:      42,
		Type:         job.TypePreparePreviewVideo,
		Status:       job.StatusRunning,
		UpdatedAtUTC: now.Add(-2 * time.Minute),
		StartedAtUTC: &startedAt,
	}

	useCase := NewWorkerStatusUseCase(stubWorkerJobReader{running: []job.Job{running}})

	result, err := useCase.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !result.LikelyAlive {
		t.Fatalf("expected running long job to remain likely alive within grace window")
	}
	if result.CurrentJob == nil || result.CurrentJob.ID != running.ID {
		t.Fatalf("expected current job to be populated from freshest running job")
	}
}

func TestWorkerStatusStaleRunningJobBecomesUnhealthyAfterGrace(t *testing.T) {
	now := time.Now().UTC()
	startedAt := now.Add(-45 * time.Minute)
	running := job.Job{
		ID:           12,
		MediaID:      43,
		Type:         job.TypePreparePreviewVideo,
		Status:       job.StatusRunning,
		UpdatedAtUTC: now.Add(-4 * time.Minute),
		StartedAtUTC: &startedAt,
	}

	useCase := NewWorkerStatusUseCase(stubWorkerJobReader{running: []job.Job{running}})

	result, err := useCase.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if result.LikelyAlive {
		t.Fatalf("expected stale running job to become unhealthy after grace window")
	}
}
