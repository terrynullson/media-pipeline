package worker

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/media"
)

func TestProcessor_RecoverInterruptedJobs(t *testing.T) {
	t.Parallel()

	jobRepo := &stubJobRepository{
		listByStatusResult: []job.Job{
			{
				ID:      10,
				MediaID: 20,
				Type:    job.TypeExtractAudio,
				Status:  job.StatusRunning,
			},
		},
	}
	mediaRepo := &stubMediaRepository{}

	processor := NewProcessor(
		jobRepo,
		mediaRepo,
		nil,
		"./data/uploads",
		"./data/audio",
		10*time.Second,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	if err := processor.RecoverInterruptedJobs(context.Background()); err != nil {
		t.Fatalf("RecoverInterruptedJobs() error = %v", err)
	}

	if len(jobRepo.requeued) != 1 {
		t.Fatalf("requeued jobs = %d, want 1", len(jobRepo.requeued))
	}
	if jobRepo.requeued[0].id != 10 {
		t.Fatalf("requeued job id = %d, want 10", jobRepo.requeued[0].id)
	}
	if jobRepo.requeued[0].errorMessage != "worker restarted before job completion" {
		t.Fatalf("requeue message = %q, want expected recovery message", jobRepo.requeued[0].errorMessage)
	}

	if len(mediaRepo.markUploadedIDs) != 1 {
		t.Fatalf("mark uploaded count = %d, want 1", len(mediaRepo.markUploadedIDs))
	}
	if mediaRepo.markUploadedIDs[0] != 20 {
		t.Fatalf("mark uploaded media id = %d, want 20", mediaRepo.markUploadedIDs[0])
	}
}

type stubJobRepository struct {
	listByStatusResult []job.Job
	requeued           []requeueCall
}

type requeueCall struct {
	id           int64
	errorMessage string
}

func (s *stubJobRepository) ClaimNextPending(context.Context, job.Type, time.Time) (job.Job, bool, error) {
	return job.Job{}, false, nil
}

func (s *stubJobRepository) MarkDone(context.Context, int64, time.Time) error {
	return nil
}

func (s *stubJobRepository) MarkFailed(context.Context, int64, string, time.Time) error {
	return nil
}

func (s *stubJobRepository) ListByStatus(context.Context, job.Type, job.Status) ([]job.Job, error) {
	return s.listByStatusResult, nil
}

func (s *stubJobRepository) Requeue(_ context.Context, id int64, errorMessage string, _ time.Time) error {
	s.requeued = append(s.requeued, requeueCall{id: id, errorMessage: errorMessage})
	return nil
}

type stubMediaRepository struct {
	markUploadedIDs []int64
}

func (s *stubMediaRepository) GetByID(context.Context, int64) (media.Media, error) {
	return media.Media{}, nil
}

func (s *stubMediaRepository) MarkProcessing(context.Context, int64, time.Time) error {
	return nil
}

func (s *stubMediaRepository) MarkAudioExtracted(context.Context, int64, string, time.Time) error {
	return nil
}

func (s *stubMediaRepository) MarkFailed(context.Context, int64, time.Time) error {
	return nil
}

func (s *stubMediaRepository) MarkUploaded(_ context.Context, id int64, _ time.Time) error {
	s.markUploadedIDs = append(s.markUploadedIDs, id)
	return nil
}
