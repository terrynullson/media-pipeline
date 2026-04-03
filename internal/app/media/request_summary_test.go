package mediaapp

import (
	"context"
	"errors"
	"testing"

	"media-pipeline/internal/domain/job"
	domainmedia "media-pipeline/internal/domain/media"
	"media-pipeline/internal/domain/transcript"
)

func TestRequestSummaryUseCase_RequestCreatesGenerateSummaryJob(t *testing.T) {
	t.Parallel()

	jobRepo := &stubSummaryRequestJobRepository{}
	uc := NewRequestSummaryUseCase(
		stubTranscriptMediaReader{item: domainmedia.Media{ID: 42, Status: domainmedia.StatusTranscribed}},
		stubTranscriptReader{item: transcript.Transcript{MediaID: 42, FullText: "text"}, ok: true},
		jobRepo,
	)

	result, err := uc.Request(context.Background(), 42)
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}
	if !result.Created {
		t.Fatal("Created = false, want true")
	}
	if len(jobRepo.createdJobs) != 1 {
		t.Fatalf("created jobs = %d, want 1", len(jobRepo.createdJobs))
	}
	if jobRepo.createdJobs[0].Type != job.TypeGenerateSummary {
		t.Fatalf("job type = %q, want %q", jobRepo.createdJobs[0].Type, job.TypeGenerateSummary)
	}
}

func TestRequestSummaryUseCase_RequestDoesNotDuplicateActiveJob(t *testing.T) {
	t.Parallel()

	jobRepo := &stubSummaryRequestJobRepository{
		latestJob: job.Job{ID: 7, MediaID: 42, Type: job.TypeGenerateSummary, Status: job.StatusRunning},
		hasJob:    true,
	}
	uc := NewRequestSummaryUseCase(
		stubTranscriptMediaReader{item: domainmedia.Media{ID: 42, Status: domainmedia.StatusTranscribed}},
		stubTranscriptReader{item: transcript.Transcript{MediaID: 42, FullText: "text"}, ok: true},
		jobRepo,
	)

	result, err := uc.Request(context.Background(), 42)
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}
	if !result.AlreadyInFlight {
		t.Fatal("AlreadyInFlight = false, want true")
	}
	if len(jobRepo.createdJobs) != 0 {
		t.Fatalf("created jobs = %#v, want none", jobRepo.createdJobs)
	}
}

func TestRequestSummaryUseCase_RequestFailsWhenTranscriptMissing(t *testing.T) {
	t.Parallel()

	uc := NewRequestSummaryUseCase(
		stubTranscriptMediaReader{item: domainmedia.Media{ID: 42, Status: domainmedia.StatusTranscribed}},
		stubTranscriptReader{},
		&stubSummaryRequestJobRepository{},
	)

	_, err := uc.Request(context.Background(), 42)
	if !errors.Is(err, ErrSummaryTranscriptNotReady) {
		t.Fatalf("Request() error = %v, want ErrSummaryTranscriptNotReady", err)
	}
}

type stubSummaryRequestJobRepository struct {
	createdJobs []job.Job
	latestJob   job.Job
	hasJob      bool
	err         error
}

func (s *stubSummaryRequestJobRepository) Create(_ context.Context, j job.Job) (int64, error) {
	s.createdJobs = append(s.createdJobs, j)
	return int64(len(s.createdJobs)), nil
}

func (s *stubSummaryRequestJobRepository) FindLatestByMediaAndType(_ context.Context, _ int64, _ job.Type) (job.Job, bool, error) {
	if s.err != nil {
		return job.Job{}, false, s.err
	}
	return s.latestJob, s.hasJob, nil
}
