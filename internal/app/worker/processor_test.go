package worker

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/media"
	"media-pipeline/internal/domain/ports"
	"media-pipeline/internal/domain/transcript"
)

func TestProcessor_RecoverInterruptedJobs(t *testing.T) {
	t.Parallel()

	jobRepo := &stubJobRepository{
		listByTypeAndStatus: map[job.Type][]job.Job{
			job.TypeExtractAudio: {
				{ID: 10, MediaID: 20, Type: job.TypeExtractAudio, Status: job.StatusRunning},
			},
			job.TypeTranscribe: {
				{ID: 11, MediaID: 21, Type: job.TypeTranscribe, Status: job.StatusRunning},
			},
		},
	}
	mediaRepo := &stubMediaRepository{}

	processor := newTestProcessor(jobRepo, mediaRepo, &stubTranscriptRepository{}, &stubAudioExtractor{}, &stubTranscriber{})

	if err := processor.RecoverInterruptedJobs(context.Background()); err != nil {
		t.Fatalf("RecoverInterruptedJobs() error = %v", err)
	}

	if len(jobRepo.requeued) != 2 {
		t.Fatalf("requeued jobs = %d, want 2", len(jobRepo.requeued))
	}
	if len(mediaRepo.markUploadedIDs) != 1 || mediaRepo.markUploadedIDs[0] != 20 {
		t.Fatalf("mark uploaded ids = %#v, want [20]", mediaRepo.markUploadedIDs)
	}
	if len(mediaRepo.markAudioReadyIDs) != 1 || mediaRepo.markAudioReadyIDs[0] != 21 {
		t.Fatalf("mark audio ready ids = %#v, want [21]", mediaRepo.markAudioReadyIDs)
	}
}

func TestProcessor_ProcessNextExtractAudioEnqueuesTranscribe(t *testing.T) {
	t.Parallel()

	uploadDir := t.TempDir()
	audioDir := t.TempDir()
	storedPath := filepath.ToSlash(filepath.Join("2026-04-03", "video.mp4"))
	inputPath := filepath.Join(uploadDir, filepath.FromSlash(storedPath))
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(inputPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	jobRepo := &stubJobRepository{
		claimByType: map[job.Type]claimResult{
			job.TypeExtractAudio: {
				job: job.Job{ID: 30, MediaID: 40, Type: job.TypeExtractAudio, Status: job.StatusRunning},
				ok:  true,
			},
		},
	}
	mediaRepo := &stubMediaRepository{
		mediaByID: map[int64]media.Media{
			40: {
				ID:          40,
				StoredName:  "video.mp4",
				StoragePath: storedPath,
				Status:      media.StatusUploaded,
			},
		},
	}
	audioExtractor := &stubAudioExtractor{
		output: ports.ExtractAudioOutput{
			OutputPath: filepath.ToSlash(filepath.Join("2026-04-03", "media_40_video.wav")),
		},
	}

	processor := NewProcessor(
		jobRepo,
		mediaRepo,
		&stubTranscriptRepository{},
		audioExtractor,
		&stubTranscriber{},
		uploadDir,
		audioDir,
		10*time.Second,
		10*time.Second,
		"ru",
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	processed, err := processor.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("ProcessNext() error = %v", err)
	}
	if !processed {
		t.Fatal("ProcessNext() processed = false, want true")
	}

	if len(mediaRepo.markAudioExtractedCalls) != 1 {
		t.Fatalf("mark audio extracted calls = %d, want 1", len(mediaRepo.markAudioExtractedCalls))
	}
	if len(jobRepo.createdJobs) != 1 || jobRepo.createdJobs[0].Type != job.TypeTranscribe {
		t.Fatalf("created jobs = %#v, want one transcribe job", jobRepo.createdJobs)
	}
	if len(jobRepo.markDoneIDs) != 1 || jobRepo.markDoneIDs[0] != 30 {
		t.Fatalf("mark done ids = %#v, want [30]", jobRepo.markDoneIDs)
	}
}

func TestProcessor_ProcessNextTranscribePersistsTranscript(t *testing.T) {
	t.Parallel()

	audioDir := t.TempDir()
	audioRelativePath := filepath.ToSlash(filepath.Join("2026-04-03", "media_50_audio.wav"))
	audioPath := filepath.Join(audioDir, filepath.FromSlash(audioRelativePath))
	if err := os.MkdirAll(filepath.Dir(audioPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(audioPath, []byte("audio"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	jobRepo := &stubJobRepository{
		claimByType: map[job.Type]claimResult{
			job.TypeTranscribe: {
				job: job.Job{ID: 31, MediaID: 50, Type: job.TypeTranscribe, Status: job.StatusRunning},
				ok:  true,
			},
		},
	}
	mediaRepo := &stubMediaRepository{
		mediaByID: map[int64]media.Media{
			50: {
				ID:                 50,
				ExtractedAudioPath: audioRelativePath,
				Status:             media.StatusAudioExtracted,
			},
		},
	}
	transcriptRepo := &stubTranscriptRepository{}
	confidence := 0.98
	transcriber := &stubTranscriber{
		output: ports.TranscribeOutput{
			FullText: "privet mir",
			Segments: []ports.TranscriptionSegment{
				{StartSec: 0, EndSec: 1.5, Text: "privet", Confidence: &confidence},
				{StartSec: 1.5, EndSec: 3, Text: "mir"},
			},
		},
	}

	processor := NewProcessor(
		jobRepo,
		mediaRepo,
		transcriptRepo,
		&stubAudioExtractor{},
		transcriber,
		t.TempDir(),
		audioDir,
		10*time.Second,
		10*time.Second,
		"ru",
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	processed, err := processor.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("ProcessNext() error = %v", err)
	}
	if !processed {
		t.Fatal("ProcessNext() processed = false, want true")
	}

	if len(mediaRepo.markTranscribingIDs) != 1 || mediaRepo.markTranscribingIDs[0] != 50 {
		t.Fatalf("mark transcribing ids = %#v, want [50]", mediaRepo.markTranscribingIDs)
	}
	if len(transcriptRepo.saved) != 1 {
		t.Fatalf("saved transcripts = %d, want 1", len(transcriptRepo.saved))
	}
	if transcriptRepo.saved[0].FullText != "privet mir" {
		t.Fatalf("saved full text = %q, want %q", transcriptRepo.saved[0].FullText, "privet mir")
	}
	if len(transcriptRepo.saved[0].Segments) != 2 {
		t.Fatalf("saved segments = %d, want 2", len(transcriptRepo.saved[0].Segments))
	}
	if len(mediaRepo.markTranscribedCalls) != 1 || mediaRepo.markTranscribedCalls[0].transcriptText != "privet mir" {
		t.Fatalf("mark transcribed calls = %#v, want transcript text", mediaRepo.markTranscribedCalls)
	}
	if len(jobRepo.markDoneIDs) != 1 || jobRepo.markDoneIDs[0] != 31 {
		t.Fatalf("mark done ids = %#v, want [31]", jobRepo.markDoneIDs)
	}
}

func TestProcessor_ProcessNextExtractAudioDoesNotDuplicateTranscribeJob(t *testing.T) {
	t.Parallel()

	uploadDir := t.TempDir()
	audioDir := t.TempDir()
	storedPath := filepath.ToSlash(filepath.Join("2026-04-03", "video.mp4"))
	inputPath := filepath.Join(uploadDir, filepath.FromSlash(storedPath))
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(inputPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	jobRepo := &stubJobRepository{
		claimByType: map[job.Type]claimResult{
			job.TypeExtractAudio: {
				job: job.Job{ID: 32, MediaID: 41, Type: job.TypeExtractAudio, Status: job.StatusRunning},
				ok:  true,
			},
		},
		existsActiveOrDone: map[job.Type]bool{
			job.TypeTranscribe: true,
		},
	}
	mediaRepo := &stubMediaRepository{
		mediaByID: map[int64]media.Media{
			41: {
				ID:          41,
				StoredName:  "video.mp4",
				StoragePath: storedPath,
				Status:      media.StatusUploaded,
			},
		},
	}
	audioExtractor := &stubAudioExtractor{
		output: ports.ExtractAudioOutput{
			OutputPath: filepath.ToSlash(filepath.Join("2026-04-03", "media_41_video.wav")),
		},
	}

	processor := NewProcessor(
		jobRepo,
		mediaRepo,
		&stubTranscriptRepository{},
		audioExtractor,
		&stubTranscriber{},
		uploadDir,
		audioDir,
		10*time.Second,
		10*time.Second,
		"ru",
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	processed, err := processor.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("ProcessNext() error = %v", err)
	}
	if !processed {
		t.Fatal("ProcessNext() processed = false, want true")
	}
	if len(jobRepo.createdJobs) != 0 {
		t.Fatalf("created jobs = %#v, want no duplicate transcribe job", jobRepo.createdJobs)
	}
}

func newTestProcessor(
	jobRepo *stubJobRepository,
	mediaRepo *stubMediaRepository,
	transcriptRepo *stubTranscriptRepository,
	audioExtractor *stubAudioExtractor,
	transcriber *stubTranscriber,
) *Processor {
	return NewProcessor(
		jobRepo,
		mediaRepo,
		transcriptRepo,
		audioExtractor,
		transcriber,
		"./data/uploads",
		"./data/audio",
		10*time.Second,
		10*time.Second,
		"",
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
}

type stubJobRepository struct {
	claimByType         map[job.Type]claimResult
	listByTypeAndStatus map[job.Type][]job.Job
	createdJobs         []job.Job
	existsActiveOrDone  map[job.Type]bool
	markDoneIDs         []int64
	markFailedCalls     []markFailedCall
	requeued            []requeueCall
}

type claimResult struct {
	job job.Job
	ok  bool
	err error
}

type requeueCall struct {
	id           int64
	errorMessage string
}

type markFailedCall struct {
	id           int64
	errorMessage string
}

func (s *stubJobRepository) Create(_ context.Context, j job.Job) (int64, error) {
	s.createdJobs = append(s.createdJobs, j)
	return int64(len(s.createdJobs)), nil
}

func (s *stubJobRepository) ExistsActiveOrDone(_ context.Context, _ int64, jobType job.Type) (bool, error) {
	return s.existsActiveOrDone[jobType], nil
}

func (s *stubJobRepository) ClaimNextPending(_ context.Context, jobType job.Type, _ time.Time) (job.Job, bool, error) {
	result, ok := s.claimByType[jobType]
	if !ok {
		return job.Job{}, false, nil
	}
	return result.job, result.ok, result.err
}

func (s *stubJobRepository) MarkDone(_ context.Context, id int64, _ time.Time) error {
	s.markDoneIDs = append(s.markDoneIDs, id)
	return nil
}

func (s *stubJobRepository) MarkFailed(_ context.Context, id int64, errorMessage string, _ time.Time) error {
	s.markFailedCalls = append(s.markFailedCalls, markFailedCall{id: id, errorMessage: errorMessage})
	return nil
}

func (s *stubJobRepository) ListByStatus(_ context.Context, jobType job.Type, _ job.Status) ([]job.Job, error) {
	return s.listByTypeAndStatus[jobType], nil
}

func (s *stubJobRepository) Requeue(_ context.Context, id int64, errorMessage string, _ time.Time) error {
	s.requeued = append(s.requeued, requeueCall{id: id, errorMessage: errorMessage})
	return nil
}

type stubMediaRepository struct {
	mediaByID               map[int64]media.Media
	markUploadedIDs         []int64
	markAudioReadyIDs       []int64
	markTranscribingIDs     []int64
	markAudioExtractedCalls []markAudioExtractedCall
	markTranscribedCalls    []markTranscribedCall
	markFailedIDs           []int64
}

type markAudioExtractedCall struct {
	id   int64
	path string
}

type markTranscribedCall struct {
	id             int64
	transcriptText string
}

func (s *stubMediaRepository) GetByID(_ context.Context, id int64) (media.Media, error) {
	return s.mediaByID[id], nil
}

func (s *stubMediaRepository) MarkProcessing(context.Context, int64, time.Time) error {
	return nil
}

func (s *stubMediaRepository) MarkAudioExtracted(_ context.Context, id int64, path string, _ time.Time) error {
	s.markAudioExtractedCalls = append(s.markAudioExtractedCalls, markAudioExtractedCall{id: id, path: path})
	return nil
}

func (s *stubMediaRepository) MarkAudioReady(_ context.Context, id int64, _ time.Time) error {
	s.markAudioReadyIDs = append(s.markAudioReadyIDs, id)
	return nil
}

func (s *stubMediaRepository) MarkTranscribing(_ context.Context, id int64, _ time.Time) error {
	s.markTranscribingIDs = append(s.markTranscribingIDs, id)
	return nil
}

func (s *stubMediaRepository) MarkTranscribed(_ context.Context, id int64, transcriptText string, _ time.Time) error {
	s.markTranscribedCalls = append(s.markTranscribedCalls, markTranscribedCall{id: id, transcriptText: transcriptText})
	return nil
}

func (s *stubMediaRepository) MarkFailed(_ context.Context, id int64, _ time.Time) error {
	s.markFailedIDs = append(s.markFailedIDs, id)
	return nil
}

func (s *stubMediaRepository) MarkUploaded(_ context.Context, id int64, _ time.Time) error {
	s.markUploadedIDs = append(s.markUploadedIDs, id)
	return nil
}

type stubTranscriptRepository struct {
	saved []transcript.Transcript
}

func (s *stubTranscriptRepository) Save(_ context.Context, item transcript.Transcript) error {
	s.saved = append(s.saved, item)
	return nil
}

type stubAudioExtractor struct {
	output ports.ExtractAudioOutput
	err    error
}

func (s *stubAudioExtractor) Extract(context.Context, ports.ExtractAudioInput) (ports.ExtractAudioOutput, error) {
	return s.output, s.err
}

type stubTranscriber struct {
	output ports.TranscribeOutput
	err    error
}

func (s *stubTranscriber) Transcribe(context.Context, ports.TranscribeInput) (ports.TranscribeOutput, error) {
	return s.output, s.err
}
