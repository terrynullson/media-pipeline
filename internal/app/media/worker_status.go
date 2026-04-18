package mediaapp

import (
	"context"
	"time"

	"media-pipeline/internal/domain/job"
)

// workerJobReader is the narrow interface required by WorkerStatusUseCase.
type workerJobReader interface {
	ListAllByStatus(ctx context.Context, status job.Status) ([]job.Job, error)
}

// WorkerStatusResult holds the full worker status snapshot.
type WorkerStatusResult struct {
	WorkerHeartbeatAge int64 // seconds since last running-job update
	LikelyAlive        bool
	CurrentJob         *WorkerCurrentJob
	Queue              WorkerQueue
}

// WorkerCurrentJob describes the currently-running job (if any).
type WorkerCurrentJob struct {
	ID              int64
	MediaID         int64
	Type            string
	StartedAt       string // RFC3339
	ProgressPercent *int
	ProgressLabel   string
}

// WorkerQueue holds pending job counts.
type WorkerQueue struct {
	Pending int
	ByType  map[string]int
}

// WorkerStatusUseCase loads the live worker state from the job table.
type WorkerStatusUseCase struct {
	jobReader workerJobReader
}

func NewWorkerStatusUseCase(jobReader workerJobReader) *WorkerStatusUseCase {
	return &WorkerStatusUseCase{jobReader: jobReader}
}

func (u *WorkerStatusUseCase) Load(ctx context.Context) (WorkerStatusResult, error) {
	running, err := u.jobReader.ListAllByStatus(ctx, job.StatusRunning)
	if err != nil {
		return WorkerStatusResult{}, err
	}
	pending, err := u.jobReader.ListAllByStatus(ctx, job.StatusPending)
	if err != nil {
		return WorkerStatusResult{}, err
	}

	var result WorkerStatusResult

	// Compute heartbeat from the most-recently-updated running job.
	now := time.Now().UTC()
	var latestUpdate time.Time
	for _, j := range running {
		if j.UpdatedAtUTC.After(latestUpdate) {
			latestUpdate = j.UpdatedAtUTC
		}
	}
	if !latestUpdate.IsZero() {
		result.WorkerHeartbeatAge = int64(now.Sub(latestUpdate).Seconds())
		result.LikelyAlive = result.WorkerHeartbeatAge <= 30
	} else {
		// No running job — check if any pending job was updated recently.
		for _, j := range pending {
			if j.UpdatedAtUTC.After(latestUpdate) {
				latestUpdate = j.UpdatedAtUTC
			}
		}
		if !latestUpdate.IsZero() {
			result.WorkerHeartbeatAge = int64(now.Sub(latestUpdate).Seconds())
		}
		result.LikelyAlive = false
	}

	// Pick the first running job as the current one.
	if len(running) > 0 {
		j := running[0]
		current := &WorkerCurrentJob{
			ID:            j.ID,
			MediaID:       j.MediaID,
			Type:          string(j.Type),
			ProgressLabel: j.ProgressLabel,
		}
		if j.StartedAtUTC != nil {
			current.StartedAt = j.StartedAtUTC.UTC().Format(time.RFC3339)
		}
		if j.ProgressPercent != nil {
			v := int(*j.ProgressPercent)
			current.ProgressPercent = &v
		}
		result.CurrentJob = current
	}

	// Build pending counts.
	byType := make(map[string]int)
	for _, j := range pending {
		byType[string(j.Type)]++
	}
	result.Queue = WorkerQueue{
		Pending: len(pending),
		ByType:  byType,
	}

	return result, nil
}
