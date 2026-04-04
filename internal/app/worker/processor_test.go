package worker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/media"
	"media-pipeline/internal/domain/ports"
	domainsummary "media-pipeline/internal/domain/summary"
	"media-pipeline/internal/domain/transcript"
	"media-pipeline/internal/domain/transcription"
	domaintrigger "media-pipeline/internal/domain/trigger"
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
		&stubTriggerRuleRepository{},
		&stubTriggerEventRepository{},
		&stubTriggerScreenshotRepository{},
		&stubSummaryRepository{},
		audioExtractor,
		&stubPreviewVideoGenerator{},
		&stubAudioDurationReader{duration: time.Minute},
		&stubScreenshotExtractor{},
		&stubTranscriber{},
		&stubSummarizer{},
		&stubTranscriptionProfileProvider{profile: transcription.DefaultProfile("ru")},
		uploadDir,
		audioDir,
		t.TempDir(),
		t.TempDir(),
		10*time.Second,
		10*time.Second,
		10*time.Second,
		10*time.Second,
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
	payload, err := job.DecodeTranscribePayload(jobRepo.createdJobs[0].Payload)
	if err != nil {
		t.Fatalf("DecodeTranscribePayload() error = %v", err)
	}
	if payload.Settings.ModelName != "tiny" || payload.Settings.Device != "cpu" {
		t.Fatalf("transcribe payload settings = %#v, want default profile snapshot", payload.Settings)
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
				job: job.Job{
					ID:      31,
					MediaID: 50,
					Type:    job.TypeTranscribe,
					Payload: mustEncodeTranscribePayload(t, job.TranscribePayload{
						Settings: transcription.Settings{
							Backend:     transcription.BackendFasterWhisper,
							ModelName:   "base",
							Device:      "cpu",
							ComputeType: "int8",
							Language:    "ru",
							BeamSize:    4,
							VADEnabled:  true,
						},
					}),
					Status: job.StatusRunning,
				},
				ok: true,
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
		&stubTriggerRuleRepository{},
		&stubTriggerEventRepository{},
		&stubTriggerScreenshotRepository{},
		&stubSummaryRepository{},
		&stubAudioExtractor{},
		&stubPreviewVideoGenerator{},
		&stubAudioDurationReader{duration: 3 * time.Second},
		&stubScreenshotExtractor{},
		transcriber,
		&stubSummarizer{},
		&stubTranscriptionProfileProvider{profile: transcription.DefaultProfile("")},
		t.TempDir(),
		audioDir,
		t.TempDir(),
		t.TempDir(),
		10*time.Second,
		10*time.Second,
		10*time.Second,
		10*time.Second,
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
	if transcriber.lastInput.Settings.ModelName != "base" || transcriber.lastInput.Settings.BeamSize != 4 {
		t.Fatalf("transcriber input settings = %#v, want payload settings", transcriber.lastInput.Settings)
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
	if len(jobRepo.createdJobs) != 1 || jobRepo.createdJobs[0].Type != job.TypeAnalyzeTriggers {
		t.Fatalf("created jobs = %#v, want one analyze_triggers job", jobRepo.createdJobs)
	}
	if len(jobRepo.markDoneIDs) != 1 || jobRepo.markDoneIDs[0] != 31 {
		t.Fatalf("mark done ids = %#v, want [31]", jobRepo.markDoneIDs)
	}
}

func TestProcessor_ProcessNextTranscribePersistsEstimatedProgress(t *testing.T) {
	t.Parallel()

	audioDir := t.TempDir()
	audioRelativePath := filepath.ToSlash(filepath.Join("2026-04-03", "media_501_audio.wav"))
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
				job: job.Job{
					ID:      301,
					MediaID: 501,
					Type:    job.TypeTranscribe,
					Payload: mustEncodeTranscribePayload(t, job.TranscribePayload{
						Settings: transcription.Settings{
							Backend:     transcription.BackendFasterWhisper,
							ModelName:   "base",
							Device:      "cpu",
							ComputeType: "int8",
							Language:    "ru",
							BeamSize:    4,
							VADEnabled:  true,
						},
					}),
					Status: job.StatusRunning,
				},
				ok: true,
			},
		},
	}
	mediaRepo := &stubMediaRepository{
		mediaByID: map[int64]media.Media{
			501: {
				ID:                 501,
				ExtractedAudioPath: audioRelativePath,
				Status:             media.StatusAudioExtracted,
			},
		},
	}
	transcriber := &stubTranscriber{
		progress: []ports.TranscriptionProgress{
			{ProcessedSec: 30, TotalSec: 60, Percent: 50, IsEstimate: true},
		},
		output: ports.TranscribeOutput{
			FullText: "privet",
			Segments: []ports.TranscriptionSegment{
				{StartSec: 0, EndSec: 1, Text: "privet"},
			},
		},
	}

	processor := NewProcessor(
		jobRepo,
		mediaRepo,
		&stubTranscriptRepository{},
		&stubTriggerRuleRepository{},
		&stubTriggerEventRepository{},
		&stubTriggerScreenshotRepository{},
		&stubSummaryRepository{},
		&stubAudioExtractor{},
		&stubPreviewVideoGenerator{},
		&stubAudioDurationReader{duration: time.Minute},
		&stubScreenshotExtractor{},
		transcriber,
		&stubSummarizer{},
		&stubTranscriptionProfileProvider{profile: transcription.DefaultProfile("")},
		t.TempDir(),
		audioDir,
		t.TempDir(),
		t.TempDir(),
		10*time.Second,
		10*time.Second,
		10*time.Second,
		10*time.Second,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	processed, err := processor.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("ProcessNext() error = %v", err)
	}
	if !processed {
		t.Fatal("ProcessNext() processed = false, want true")
	}
	if len(jobRepo.progressUpdates) != 1 {
		t.Fatalf("progress updates = %d, want 1", len(jobRepo.progressUpdates))
	}
	if jobRepo.progressUpdates[0].percent == nil || *jobRepo.progressUpdates[0].percent != 50 {
		t.Fatalf("progress percent = %#v, want 50", jobRepo.progressUpdates[0].percent)
	}
	if !jobRepo.progressUpdates[0].isEstimate {
		t.Fatal("progress update should be marked as estimate")
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
		&stubTriggerRuleRepository{},
		&stubTriggerEventRepository{},
		&stubTriggerScreenshotRepository{},
		&stubSummaryRepository{},
		audioExtractor,
		&stubPreviewVideoGenerator{},
		&stubAudioDurationReader{duration: time.Minute},
		&stubScreenshotExtractor{},
		&stubTranscriber{},
		&stubSummarizer{},
		&stubTranscriptionProfileProvider{profile: transcription.DefaultProfile("ru")},
		uploadDir,
		audioDir,
		t.TempDir(),
		t.TempDir(),
		10*time.Second,
		10*time.Second,
		10*time.Second,
		10*time.Second,
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

func TestProcessor_ProcessNextTranscribeFailureStoresReadableError(t *testing.T) {
	t.Parallel()

	audioDir := t.TempDir()
	audioRelativePath := filepath.ToSlash(filepath.Join("2026-04-03", "media_51_audio.wav"))
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
				job: job.Job{
					ID:      41,
					MediaID: 51,
					Type:    job.TypeTranscribe,
					Payload: mustEncodeTranscribePayload(t, job.TranscribePayload{
						Settings: transcription.Settings{
							Backend:     transcription.BackendFasterWhisper,
							ModelName:   "small",
							Device:      "cpu",
							ComputeType: "int8",
							Language:    "ru",
							BeamSize:    3,
							VADEnabled:  true,
						},
					}),
					Status: job.StatusRunning,
				},
				ok: true,
			},
		},
	}
	mediaRepo := &stubMediaRepository{
		mediaByID: map[int64]media.Media{
			51: {
				ID:                 51,
				ExtractedAudioPath: audioRelativePath,
				Status:             media.StatusAudioExtracted,
			},
		},
	}
	transcriber := &stubTranscriber{
		err: &ports.TranscriptionError{
			Cause:       errors.New("run python transcription: exit status 1"),
			Diagnostics: "RuntimeError: transcription backend returned empty text",
		},
	}

	processor := NewProcessor(
		jobRepo,
		mediaRepo,
		&stubTranscriptRepository{},
		&stubTriggerRuleRepository{},
		&stubTriggerEventRepository{},
		&stubTriggerScreenshotRepository{},
		&stubSummaryRepository{},
		&stubAudioExtractor{},
		&stubPreviewVideoGenerator{},
		&stubAudioDurationReader{duration: 2 * time.Minute},
		&stubScreenshotExtractor{},
		transcriber,
		&stubSummarizer{},
		&stubTranscriptionProfileProvider{profile: transcription.DefaultProfile("")},
		t.TempDir(),
		audioDir,
		t.TempDir(),
		t.TempDir(),
		10*time.Second,
		10*time.Second,
		10*time.Second,
		10*time.Second,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	processed, err := processor.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("ProcessNext() error = %v", err)
	}
	if !processed {
		t.Fatal("ProcessNext() processed = false, want true")
	}
	if len(jobRepo.markFailedCalls) != 1 {
		t.Fatalf("mark failed calls = %d, want 1", len(jobRepo.markFailedCalls))
	}
	if got := jobRepo.markFailedCalls[0].errorMessage; got != "Не удалось распознать текст: модель вернула пустой результат" {
		t.Fatalf("errorMessage = %q, want readable transcription failure", got)
	}
	if len(mediaRepo.markFailedIDs) != 1 || mediaRepo.markFailedIDs[0] != 51 {
		t.Fatalf("mark failed ids = %#v, want [51]", mediaRepo.markFailedIDs)
	}
}

func TestProcessor_ProcessNextTranscribeFailureDoesNotExposeBarePythonReason(t *testing.T) {
	t.Parallel()

	audioDir := t.TempDir()
	audioRelativePath := filepath.ToSlash(filepath.Join("2026-04-03", "media_51_audio.wav"))
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
				job: job.Job{
					ID:      42,
					MediaID: 52,
					Type:    job.TypeTranscribe,
					Payload: mustEncodeTranscribePayload(t, job.TranscribePayload{
						Settings: transcription.Settings{
							Backend:     transcription.BackendFasterWhisper,
							ModelName:   "small",
							Device:      "cuda",
							ComputeType: "int8_float32",
							Language:    "ru",
							BeamSize:    5,
							VADEnabled:  true,
						},
					}),
					Status: job.StatusRunning,
				},
				ok: true,
			},
		},
	}
	mediaRepo := &stubMediaRepository{
		mediaByID: map[int64]media.Media{
			52: {
				ID:                 52,
				ExtractedAudioPath: audioRelativePath,
				Status:             media.StatusAudioExtracted,
			},
		},
	}
	transcriber := &stubTranscriber{
		err: &ports.TranscriptionError{
			Cause:       errors.New("run python transcription: exit status 9009"),
			Diagnostics: "Python",
		},
	}

	processor := NewProcessor(
		jobRepo,
		mediaRepo,
		&stubTranscriptRepository{},
		&stubTriggerRuleRepository{},
		&stubTriggerEventRepository{},
		&stubTriggerScreenshotRepository{},
		&stubSummaryRepository{},
		&stubAudioExtractor{},
		&stubPreviewVideoGenerator{},
		&stubAudioDurationReader{duration: 2 * time.Minute},
		&stubScreenshotExtractor{},
		transcriber,
		&stubSummarizer{},
		&stubTranscriptionProfileProvider{profile: transcription.DefaultProfile("")},
		t.TempDir(),
		audioDir,
		t.TempDir(),
		t.TempDir(),
		10*time.Second,
		10*time.Second,
		10*time.Second,
		10*time.Second,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	processed, err := processor.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("ProcessNext() error = %v", err)
	}
	if !processed {
		t.Fatal("ProcessNext() processed = false, want true")
	}
	if len(jobRepo.markFailedCalls) != 1 {
		t.Fatalf("mark failed calls = %d, want 1", len(jobRepo.markFailedCalls))
	}
	got := jobRepo.markFailedCalls[0].errorMessage
	if strings.Contains(strings.ToLower(got), "python") {
		t.Fatalf("errorMessage = %q, want user-facing message without bare Python", got)
	}
}

func TestProcessor_ProcessNextTranscribeLongSmallCPUUsesAdaptiveTimeout(t *testing.T) {
	t.Parallel()

	audioDir := t.TempDir()
	audioRelativePath := filepath.ToSlash(filepath.Join("2026-04-03", "media_52_audio.wav"))
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
				job: job.Job{
					ID:      42,
					MediaID: 52,
					Type:    job.TypeTranscribe,
					Payload: mustEncodeTranscribePayload(t, job.TranscribePayload{
						Settings: transcription.Settings{
							Backend:     transcription.BackendFasterWhisper,
							ModelName:   "small",
							Device:      "cpu",
							ComputeType: "int8",
							Language:    "ru",
							BeamSize:    5,
							VADEnabled:  true,
						},
					}),
					Status: job.StatusRunning,
				},
				ok: true,
			},
		},
	}
	mediaRepo := &stubMediaRepository{
		mediaByID: map[int64]media.Media{
			52: {
				ID:                 52,
				ExtractedAudioPath: audioRelativePath,
				Status:             media.StatusAudioExtracted,
			},
		},
	}
	transcriber := &stubTranscriber{
		output: ports.TranscribeOutput{
			FullText: "privet",
			Segments: []ports.TranscriptionSegment{
				{StartSec: 0, EndSec: 1, Text: "privet"},
			},
		},
	}

	processor := NewProcessor(
		jobRepo,
		mediaRepo,
		&stubTranscriptRepository{},
		&stubTriggerRuleRepository{},
		&stubTriggerEventRepository{},
		&stubTriggerScreenshotRepository{},
		&stubSummaryRepository{},
		&stubAudioExtractor{},
		&stubPreviewVideoGenerator{},
		&stubAudioDurationReader{duration: 60 * time.Minute},
		&stubScreenshotExtractor{},
		transcriber,
		&stubSummarizer{},
		&stubTranscriptionProfileProvider{profile: transcription.DefaultProfile("")},
		t.TempDir(),
		audioDir,
		t.TempDir(),
		t.TempDir(),
		10*time.Second,
		10*time.Second,
		5*time.Minute,
		10*time.Second,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	processed, err := processor.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("ProcessNext() error = %v", err)
	}
	if !processed {
		t.Fatal("ProcessNext() processed = false, want true")
	}
	if !transcriber.deadlineSet {
		t.Fatal("transcriber deadline was not set")
	}
	if remaining := time.Until(transcriber.deadline); remaining < 9*time.Hour {
		t.Fatalf("adaptive timeout = %s, want at least 9h", remaining)
	}
}

func TestProcessor_ProcessNextTranscribeBlocksUnrealisticPolicy(t *testing.T) {
	t.Parallel()

	audioDir := t.TempDir()
	audioRelativePath := filepath.ToSlash(filepath.Join("2026-04-03", "media_53_audio.wav"))
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
				job: job.Job{
					ID:      43,
					MediaID: 53,
					Type:    job.TypeTranscribe,
					Payload: mustEncodeTranscribePayload(t, job.TranscribePayload{
						Settings: transcription.Settings{
							Backend:     transcription.BackendFasterWhisper,
							ModelName:   "small",
							Device:      "cpu",
							ComputeType: "int8",
							Language:    "ru",
							BeamSize:    5,
							VADEnabled:  true,
						},
					}),
					Status: job.StatusRunning,
				},
				ok: true,
			},
		},
	}
	mediaRepo := &stubMediaRepository{
		mediaByID: map[int64]media.Media{
			53: {
				ID:                 53,
				ExtractedAudioPath: audioRelativePath,
				Status:             media.StatusAudioExtracted,
			},
		},
	}
	transcriber := &stubTranscriber{}

	processor := NewProcessor(
		jobRepo,
		mediaRepo,
		&stubTranscriptRepository{},
		&stubTriggerRuleRepository{},
		&stubTriggerEventRepository{},
		&stubTriggerScreenshotRepository{},
		&stubSummaryRepository{},
		&stubAudioExtractor{},
		&stubPreviewVideoGenerator{},
		&stubAudioDurationReader{duration: 2 * time.Hour},
		&stubScreenshotExtractor{},
		transcriber,
		&stubSummarizer{},
		&stubTranscriptionProfileProvider{profile: transcription.DefaultProfile("")},
		t.TempDir(),
		audioDir,
		t.TempDir(),
		t.TempDir(),
		10*time.Second,
		10*time.Second,
		5*time.Minute,
		10*time.Second,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	processed, err := processor.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("ProcessNext() error = %v", err)
	}
	if !processed {
		t.Fatal("ProcessNext() processed = false, want true")
	}
	if transcriber.callCount != 0 {
		t.Fatalf("transcriber callCount = %d, want 0", transcriber.callCount)
	}
	if len(jobRepo.markFailedCalls) != 1 {
		t.Fatalf("mark failed calls = %d, want 1", len(jobRepo.markFailedCalls))
	}
	if got := jobRepo.markFailedCalls[0].errorMessage; !strings.Contains(got, "задача не запускается автоматически") {
		t.Fatalf("errorMessage = %q, want clear policy rejection", got)
	}
}

func TestProcessor_ProcessNextTranscribeTimeoutFailureIncludesPolicyContext(t *testing.T) {
	t.Parallel()

	audioDir := t.TempDir()
	audioRelativePath := filepath.ToSlash(filepath.Join("2026-04-03", "media_54_audio.wav"))
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
				job: job.Job{
					ID:      44,
					MediaID: 54,
					Type:    job.TypeTranscribe,
					Payload: mustEncodeTranscribePayload(t, job.TranscribePayload{
						Settings: transcription.Settings{
							Backend:     transcription.BackendFasterWhisper,
							ModelName:   "small",
							Device:      "cpu",
							ComputeType: "int8",
							Language:    "ru",
							BeamSize:    5,
							VADEnabled:  true,
						},
					}),
					Status: job.StatusRunning,
				},
				ok: true,
			},
		},
	}
	mediaRepo := &stubMediaRepository{
		mediaByID: map[int64]media.Media{
			54: {
				ID:                 54,
				ExtractedAudioPath: audioRelativePath,
				Status:             media.StatusAudioExtracted,
			},
		},
	}
	transcriber := &stubTranscriber{
		err: &ports.TranscriptionError{
			Cause:       fmt.Errorf("python transcription timed out: %w", context.DeadlineExceeded),
			Diagnostics: "",
		},
	}

	processor := NewProcessor(
		jobRepo,
		mediaRepo,
		&stubTranscriptRepository{},
		&stubTriggerRuleRepository{},
		&stubTriggerEventRepository{},
		&stubTriggerScreenshotRepository{},
		&stubSummaryRepository{},
		&stubAudioExtractor{},
		&stubPreviewVideoGenerator{},
		&stubAudioDurationReader{duration: 60 * time.Minute},
		&stubScreenshotExtractor{},
		transcriber,
		&stubSummarizer{},
		&stubTranscriptionProfileProvider{profile: transcription.DefaultProfile("")},
		t.TempDir(),
		audioDir,
		t.TempDir(),
		t.TempDir(),
		10*time.Second,
		10*time.Second,
		5*time.Minute,
		10*time.Second,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	processed, err := processor.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("ProcessNext() error = %v", err)
	}
	if !processed {
		t.Fatal("ProcessNext() processed = false, want true")
	}
	if len(jobRepo.markFailedCalls) != 1 {
		t.Fatalf("mark failed calls = %d, want 1", len(jobRepo.markFailedCalls))
	}
	if got := jobRepo.markFailedCalls[0].errorMessage; got != "Не удалось распознать текст: модель small на CPU (int8) для длинного файла превысила адаптивный лимит 9 ч 15 мин." {
		t.Fatalf("errorMessage = %q, want contextual timeout message", got)
	}
}

func TestProcessor_ProcessNextAnalyzeTriggersPersistsEvents(t *testing.T) {
	t.Parallel()

	jobRepo := &stubJobRepository{
		claimByType: map[job.Type]claimResult{
			job.TypeAnalyzeTriggers: {
				job: job.Job{ID: 33, MediaID: 60, Type: job.TypeAnalyzeTriggers, Status: job.StatusRunning},
				ok:  true,
			},
		},
	}
	mediaRepo := &stubMediaRepository{}
	transcriptID := int64(12)
	transcriptRepo := &stubTranscriptRepository{
		item: transcript.Transcript{
			ID:      transcriptID,
			MediaID: 60,
			Segments: []transcript.Segment{
				{StartSec: 0, EndSec: 2, Text: "Customer asked for a refund today."},
				{StartSec: 2, EndSec: 4, Text: "No escalation requested."},
			},
		},
		ok: true,
	}
	triggerRuleRepo := &stubTriggerRuleRepository{
		items: []domaintrigger.Rule{
			{ID: 7, Name: "Refund", Category: "billing", Pattern: "refund", MatchMode: domaintrigger.MatchModeContains, Enabled: true},
		},
	}
	triggerEventRepo := &stubTriggerEventRepository{}

	processor := NewProcessor(
		jobRepo,
		mediaRepo,
		transcriptRepo,
		triggerRuleRepo,
		triggerEventRepo,
		&stubTriggerScreenshotRepository{},
		&stubSummaryRepository{},
		&stubAudioExtractor{},
		&stubPreviewVideoGenerator{},
		&stubAudioDurationReader{duration: time.Minute},
		&stubScreenshotExtractor{},
		&stubTranscriber{},
		&stubSummarizer{},
		&stubTranscriptionProfileProvider{profile: transcription.DefaultProfile("")},
		t.TempDir(),
		t.TempDir(),
		t.TempDir(),
		t.TempDir(),
		10*time.Second,
		10*time.Second,
		10*time.Second,
		10*time.Second,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	processed, err := processor.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("ProcessNext() error = %v", err)
	}
	if !processed {
		t.Fatal("ProcessNext() processed = false, want true")
	}
	if len(triggerEventRepo.replaceCalls) != 1 {
		t.Fatalf("replace calls = %d, want 1", len(triggerEventRepo.replaceCalls))
	}
	if got := len(triggerEventRepo.replaceCalls[0].events); got != 1 {
		t.Fatalf("saved trigger events = %d, want 1", got)
	}
	if triggerEventRepo.replaceCalls[0].events[0].MatchedText != "refund" {
		t.Fatalf("matched text = %q, want %q", triggerEventRepo.replaceCalls[0].events[0].MatchedText, "refund")
	}
	if len(jobRepo.createdJobs) != 1 || jobRepo.createdJobs[0].Type != job.TypeExtractScreenshots {
		t.Fatalf("created jobs = %#v, want one extract_screenshots job", jobRepo.createdJobs)
	}
	if len(jobRepo.markDoneIDs) != 1 || jobRepo.markDoneIDs[0] != 33 {
		t.Fatalf("mark done ids = %#v, want [33]", jobRepo.markDoneIDs)
	}
	if len(mediaRepo.markFailedIDs) != 0 {
		t.Fatalf("mark failed ids = %#v, want none", mediaRepo.markFailedIDs)
	}
}

func TestProcessor_ProcessNextExtractScreenshotsPersistsRows(t *testing.T) {
	t.Parallel()

	uploadDir := t.TempDir()
	screenshotsDir := t.TempDir()
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
			job.TypeExtractScreenshots: {
				job: job.Job{ID: 34, MediaID: 70, Type: job.TypeExtractScreenshots, Status: job.StatusRunning},
				ok:  true,
			},
		},
	}
	mediaRepo := &stubMediaRepository{
		mediaByID: map[int64]media.Media{
			70: {
				ID:          70,
				StoragePath: storedPath,
				MIMEType:    "video/mp4",
				Extension:   ".mp4",
			},
		},
	}
	triggerEventRepo := &stubTriggerEventRepository{
		items: []domaintrigger.Event{
			{ID: 100, MediaID: 70, StartSec: 3.25, MatchedText: "refund"},
		},
	}
	triggerScreenshotRepo := &stubTriggerScreenshotRepository{}
	screenshotExtractor := &stubScreenshotExtractor{
		outputByEventID: map[int64]ports.ExtractScreenshotOutput{
			100: {
				ImagePath: filepath.ToSlash(filepath.Join("2026-04-03", "media_70_trigger_100_3250ms.jpg")),
				Width:     640,
				Height:    360,
			},
		},
	}

	processor := NewProcessor(
		jobRepo,
		mediaRepo,
		&stubTranscriptRepository{},
		&stubTriggerRuleRepository{},
		triggerEventRepo,
		triggerScreenshotRepo,
		&stubSummaryRepository{},
		&stubAudioExtractor{},
		&stubPreviewVideoGenerator{},
		&stubAudioDurationReader{duration: time.Minute},
		screenshotExtractor,
		&stubTranscriber{},
		&stubSummarizer{},
		&stubTranscriptionProfileProvider{profile: transcription.DefaultProfile("")},
		uploadDir,
		t.TempDir(),
		screenshotsDir,
		t.TempDir(),
		10*time.Second,
		10*time.Second,
		10*time.Second,
		10*time.Second,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	processed, err := processor.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("ProcessNext() error = %v", err)
	}
	if !processed {
		t.Fatal("ProcessNext() processed = false, want true")
	}
	if len(triggerScreenshotRepo.replaceCalls) != 1 {
		t.Fatalf("replace screenshot calls = %d, want 1", len(triggerScreenshotRepo.replaceCalls))
	}
	if got := len(triggerScreenshotRepo.replaceCalls[0].items); got != 1 {
		t.Fatalf("saved screenshots = %d, want 1", got)
	}
	if triggerScreenshotRepo.replaceCalls[0].items[0].TriggerEventID != 100 {
		t.Fatalf("trigger event id = %d, want 100", triggerScreenshotRepo.replaceCalls[0].items[0].TriggerEventID)
	}
	if len(jobRepo.markDoneIDs) != 1 || jobRepo.markDoneIDs[0] != 34 {
		t.Fatalf("mark done ids = %#v, want [34]", jobRepo.markDoneIDs)
	}
}

func TestProcessor_ProcessNextExtractScreenshotsSkipsAudioOnlyMedia(t *testing.T) {
	t.Parallel()

	jobRepo := &stubJobRepository{
		claimByType: map[job.Type]claimResult{
			job.TypeExtractScreenshots: {
				job: job.Job{ID: 35, MediaID: 71, Type: job.TypeExtractScreenshots, Status: job.StatusRunning},
				ok:  true,
			},
		},
	}
	mediaRepo := &stubMediaRepository{
		mediaByID: map[int64]media.Media{
			71: {ID: 71, MIMEType: "audio/wav", Extension: ".wav"},
		},
	}
	triggerScreenshotRepo := &stubTriggerScreenshotRepository{}

	processor := NewProcessor(
		jobRepo,
		mediaRepo,
		&stubTranscriptRepository{},
		&stubTriggerRuleRepository{},
		&stubTriggerEventRepository{},
		triggerScreenshotRepo,
		&stubSummaryRepository{},
		&stubAudioExtractor{},
		&stubPreviewVideoGenerator{},
		&stubAudioDurationReader{duration: time.Minute},
		&stubScreenshotExtractor{},
		&stubTranscriber{},
		&stubSummarizer{},
		&stubTranscriptionProfileProvider{profile: transcription.DefaultProfile("")},
		t.TempDir(),
		t.TempDir(),
		t.TempDir(),
		t.TempDir(),
		10*time.Second,
		10*time.Second,
		10*time.Second,
		10*time.Second,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	processed, err := processor.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("ProcessNext() error = %v", err)
	}
	if !processed {
		t.Fatal("ProcessNext() processed = false, want true")
	}
	if len(triggerScreenshotRepo.replaceCalls) != 1 {
		t.Fatalf("replace screenshot calls = %d, want 1", len(triggerScreenshotRepo.replaceCalls))
	}
	if got := len(triggerScreenshotRepo.replaceCalls[0].items); got != 0 {
		t.Fatalf("saved screenshots = %d, want 0", got)
	}
}

func TestProcessor_ProcessNextGenerateSummaryPersistsSummary(t *testing.T) {
	t.Parallel()

	jobRepo := &stubJobRepository{
		claimByType: map[job.Type]claimResult{
			job.TypeGenerateSummary: {
				job: job.Job{ID: 36, MediaID: 72, Type: job.TypeGenerateSummary, Status: job.StatusRunning},
				ok:  true,
			},
		},
	}
	transcriptRepo := &stubTranscriptRepository{
		item: transcript.Transcript{
			ID:       9,
			MediaID:  72,
			FullText: "Клиент рассказал о проблеме. Затем попросил возврат средств.",
			Segments: []transcript.Segment{
				{StartSec: 0, EndSec: 2, Text: "Клиент рассказал о проблеме."},
				{StartSec: 2, EndSec: 4, Text: "Затем попросил возврат средств."},
			},
		},
		ok: true,
	}
	triggerEventRepo := &stubTriggerEventRepository{
		items: []domaintrigger.Event{
			{ID: 11, MediaID: 72, MatchedText: "refund", Category: "billing", StartSec: 2},
		},
	}
	triggerScreenshotRepo := &stubTriggerScreenshotRepository{
		items: []domaintrigger.Screenshot{
			{ID: 14, MediaID: 72, TriggerEventID: 11, ImagePath: "2026-04-03/s1.jpg"},
		},
	}
	summaryRepo := &stubSummaryRepository{}
	summarizer := &stubSummarizer{
		output: ports.SummaryOutput{
			SummaryText: "Короткое саммари разговора.",
			Highlights:  []string{"Клиент описал проблему.", "Попросил возврат средств."},
			Provider:    "simple-summary-v1",
		},
	}

	processor := NewProcessor(
		jobRepo,
		&stubMediaRepository{},
		transcriptRepo,
		&stubTriggerRuleRepository{},
		triggerEventRepo,
		triggerScreenshotRepo,
		summaryRepo,
		&stubAudioExtractor{},
		&stubPreviewVideoGenerator{},
		&stubAudioDurationReader{duration: time.Minute},
		&stubScreenshotExtractor{},
		&stubTranscriber{},
		summarizer,
		&stubTranscriptionProfileProvider{profile: transcription.DefaultProfile("")},
		t.TempDir(),
		t.TempDir(),
		t.TempDir(),
		t.TempDir(),
		10*time.Second,
		10*time.Second,
		10*time.Second,
		10*time.Second,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	processed, err := processor.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("ProcessNext() error = %v", err)
	}
	if !processed {
		t.Fatal("ProcessNext() processed = false, want true")
	}
	if summarizer.lastInput.MediaID != 72 {
		t.Fatalf("summarizer media id = %d, want 72", summarizer.lastInput.MediaID)
	}
	if len(summaryRepo.saved) != 1 {
		t.Fatalf("saved summaries = %d, want 1", len(summaryRepo.saved))
	}
	if summaryRepo.saved[0].SummaryText != "Короткое саммари разговора." {
		t.Fatalf("SummaryText = %q, want persisted summary", summaryRepo.saved[0].SummaryText)
	}
	if len(jobRepo.markDoneIDs) != 1 || jobRepo.markDoneIDs[0] != 36 {
		t.Fatalf("mark done ids = %#v, want [36]", jobRepo.markDoneIDs)
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
		&stubTriggerRuleRepository{},
		&stubTriggerEventRepository{},
		&stubTriggerScreenshotRepository{},
		&stubSummaryRepository{},
		audioExtractor,
		&stubPreviewVideoGenerator{},
		&stubAudioDurationReader{duration: time.Minute},
		&stubScreenshotExtractor{},
		transcriber,
		&stubSummarizer{},
		&stubTranscriptionProfileProvider{profile: transcription.DefaultProfile("")},
		"./data/uploads",
		"./data/audio",
		"./data/previews",
		"./data/screenshots",
		10*time.Second,
		10*time.Second,
		10*time.Second,
		10*time.Second,
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
	progressUpdates     []progressUpdateCall
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

type progressUpdateCall struct {
	id         int64
	percent    *float64
	label      string
	isEstimate bool
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

func (s *stubJobRepository) UpdateProgress(_ context.Context, id int64, progressPercent *float64, progressLabel string, isEstimate bool, _ time.Time) error {
	s.progressUpdates = append(s.progressUpdates, progressUpdateCall{
		id:         id,
		percent:    progressPercent,
		label:      progressLabel,
		isEstimate: isEstimate,
	})
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
	markPreviewReadyCalls   []markPreviewReadyCall
	markTranscribedCalls    []markTranscribedCall
	markFailedIDs           []int64
}

type markAudioExtractedCall struct {
	id   int64
	path string
}

type markPreviewReadyCall struct {
	id       int64
	path     string
	size     int64
	mimeType string
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

func (s *stubMediaRepository) MarkPreviewReady(_ context.Context, id int64, path string, sizeBytes int64, mimeType string, _ time.Time, _ time.Time) error {
	s.markPreviewReadyCalls = append(s.markPreviewReadyCalls, markPreviewReadyCall{id: id, path: path, size: sizeBytes, mimeType: mimeType})
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
	item  transcript.Transcript
	ok    bool
	err   error
}

func (s *stubTranscriptRepository) Save(_ context.Context, item transcript.Transcript) error {
	s.saved = append(s.saved, item)
	return nil
}

func (s *stubTranscriptRepository) GetByMediaID(_ context.Context, _ int64) (transcript.Transcript, bool, error) {
	if s.err != nil {
		return transcript.Transcript{}, false, s.err
	}
	if s.ok {
		return s.item, true, nil
	}
	if len(s.saved) > 0 {
		return s.saved[len(s.saved)-1], true, nil
	}
	return transcript.Transcript{}, false, nil
}

type stubTriggerRuleRepository struct {
	items []domaintrigger.Rule
	err   error
}

func (s *stubTriggerRuleRepository) ListEnabled(context.Context) ([]domaintrigger.Rule, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.items, nil
}

type replaceTriggerEventsCall struct {
	mediaID      int64
	transcriptID *int64
	events       []domaintrigger.Event
}

type stubTriggerEventRepository struct {
	replaceCalls []replaceTriggerEventsCall
	items        []domaintrigger.Event
	err          error
}

func (s *stubTriggerEventRepository) ReplaceForMedia(_ context.Context, mediaID int64, transcriptID *int64, events []domaintrigger.Event) error {
	if s.err != nil {
		return s.err
	}
	s.replaceCalls = append(s.replaceCalls, replaceTriggerEventsCall{
		mediaID:      mediaID,
		transcriptID: transcriptID,
		events:       append([]domaintrigger.Event(nil), events...),
	})
	return nil
}

func (s *stubTriggerEventRepository) ListByMediaID(context.Context, int64) ([]domaintrigger.Event, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.items, nil
}

type replaceTriggerScreenshotsCall struct {
	mediaID int64
	items   []domaintrigger.Screenshot
}

type stubTriggerScreenshotRepository struct {
	replaceCalls []replaceTriggerScreenshotsCall
	items        []domaintrigger.Screenshot
	paths        []string
	err          error
}

func (s *stubTriggerScreenshotRepository) ReplaceForMedia(_ context.Context, mediaID int64, items []domaintrigger.Screenshot) error {
	if s.err != nil {
		return s.err
	}
	s.replaceCalls = append(s.replaceCalls, replaceTriggerScreenshotsCall{
		mediaID: mediaID,
		items:   append([]domaintrigger.Screenshot(nil), items...),
	})
	return nil
}

func (s *stubTriggerScreenshotRepository) ListPathsByMediaID(context.Context, int64) ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]string(nil), s.paths...), nil
}

func (s *stubTriggerScreenshotRepository) ListByMediaID(context.Context, int64) ([]domaintrigger.Screenshot, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]domaintrigger.Screenshot(nil), s.items...), nil
}

type stubSummaryRepository struct {
	saved []domainsummary.Summary
	err   error
}

func (s *stubSummaryRepository) Save(_ context.Context, item domainsummary.Summary) error {
	if s.err != nil {
		return s.err
	}
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

type stubPreviewVideoGenerator struct {
	output ports.GeneratePreviewVideoOutput
	err    error
}

func (s *stubPreviewVideoGenerator) Generate(context.Context, ports.GeneratePreviewVideoInput) (ports.GeneratePreviewVideoOutput, error) {
	return s.output, s.err
}

type stubAudioDurationReader struct {
	duration time.Duration
	err      error
	lastPath string
}

func (s *stubAudioDurationReader) ReadDuration(audioPath string) (time.Duration, error) {
	s.lastPath = audioPath
	if s.err != nil {
		return 0, s.err
	}
	return s.duration, nil
}

type stubScreenshotExtractor struct {
	outputByEventID map[int64]ports.ExtractScreenshotOutput
	err             error
}

func (s *stubScreenshotExtractor) Extract(_ context.Context, in ports.ExtractScreenshotInput) (ports.ExtractScreenshotOutput, error) {
	if s.err != nil {
		return ports.ExtractScreenshotOutput{}, s.err
	}
	if s.outputByEventID != nil {
		if output, ok := s.outputByEventID[in.TriggerEventID]; ok {
			return output, nil
		}
	}
	return ports.ExtractScreenshotOutput{
		ImagePath: filepath.ToSlash(filepath.Join("2026-04-03", "screenshot.jpg")),
		Width:     320,
		Height:    180,
	}, nil
}

type stubTranscriber struct {
	output      ports.TranscribeOutput
	err         error
	lastInput   ports.TranscribeInput
	callCount   int
	deadline    time.Time
	deadlineSet bool
	progress    []ports.TranscriptionProgress
}

func (s *stubTranscriber) Transcribe(ctx context.Context, in ports.TranscribeInput) (ports.TranscribeOutput, error) {
	s.callCount++
	s.lastInput = in
	if deadline, ok := ctx.Deadline(); ok {
		s.deadline = deadline
		s.deadlineSet = true
	}
	for _, item := range s.progress {
		if in.Progress != nil {
			in.Progress(item)
		}
	}
	return s.output, s.err
}

type stubTranscriptionProfileProvider struct {
	profile transcription.Profile
	err     error
}

func (s *stubTranscriptionProfileProvider) GetCurrent(context.Context) (transcription.Profile, error) {
	return s.profile, s.err
}

type stubSummarizer struct {
	output    ports.SummaryOutput
	err       error
	lastInput ports.SummaryInput
}

func (s *stubSummarizer) Generate(_ context.Context, in ports.SummaryInput) (ports.SummaryOutput, error) {
	s.lastInput = in
	if s.err != nil {
		return ports.SummaryOutput{}, s.err
	}
	if strings.TrimSpace(s.output.Provider) == "" {
		s.output.Provider = "simple-summary-v1"
	}
	if strings.TrimSpace(s.output.SummaryText) == "" {
		s.output.SummaryText = "Короткое саммари."
	}
	return s.output, nil
}

func mustEncodeTranscribePayload(t *testing.T, payload job.TranscribePayload) string {
	t.Helper()

	raw, err := job.EncodeTranscribePayload(payload)
	if err != nil {
		t.Fatalf("EncodeTranscribePayload() error = %v", err)
	}

	return raw
}
