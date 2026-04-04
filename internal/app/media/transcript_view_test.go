package mediaapp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"media-pipeline/internal/domain/job"
	domainmedia "media-pipeline/internal/domain/media"
	domainsummary "media-pipeline/internal/domain/summary"
	"media-pipeline/internal/domain/transcript"
	"media-pipeline/internal/domain/transcription"
	domaintrigger "media-pipeline/internal/domain/trigger"
)

func TestTranscriptViewUseCase_LoadIncludesTranscriptAndSettings(t *testing.T) {
	t.Parallel()

	mediaItem := domainmedia.Media{
		ID:           42,
		OriginalName: "demo.wav",
		SizeBytes:    4096,
		Status:       domainmedia.StatusTranscribed,
		CreatedAtUTC: time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
	}
	transcriptItem := transcript.Transcript{
		ID:       7,
		MediaID:  42,
		FullText: "hello world",
		Segments: []transcript.Segment{
			{StartSec: 0, EndSec: 1, Text: "hello"},
			{StartSec: 1, EndSec: 2, Text: "world"},
		},
	}
	payload, err := job.EncodeTranscribePayload(job.TranscribePayload{
		Settings: transcription.Settings{
			Backend:     transcription.BackendFasterWhisper,
			ModelName:   "small",
			Device:      "cpu",
			ComputeType: "int8",
			Language:    "ru",
			BeamSize:    4,
			VADEnabled:  true,
		},
	})
	if err != nil {
		t.Fatalf("EncodeTranscribePayload() error = %v", err)
	}

	uc := NewTranscriptViewUseCase(
		stubTranscriptMediaReader{item: mediaItem},
		stubTranscriptReader{item: transcriptItem, ok: true},
		stubTriggerEventReader{},
		stubTriggerScreenshotReader{},
		stubSummaryReader{},
		stubTranscriptJobReader{item: job.Job{MediaID: 42, Type: job.TypeTranscribe, Payload: payload}, ok: true},
		t.TempDir(),
		stubTranscriptAudioDurationReader{},
		t.TempDir(),
		t.TempDir(),
		5*time.Minute,
	)

	result, err := uc.Load(context.Background(), 42)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !result.HasTranscript {
		t.Fatal("HasTranscript = false, want true")
	}
	if result.Transcript.FullText != "hello world" {
		t.Fatalf("FullText = %q, want %q", result.Transcript.FullText, "hello world")
	}
	if result.Settings == nil {
		t.Fatal("Settings = nil, want decoded settings")
	}
	if result.Settings.ModelName != "small" || result.Settings.Language != "ru" {
		t.Fatalf("Settings = %#v, want decoded payload settings", result.Settings)
	}
}

func TestTranscriptViewUseCase_LoadKeepsWorkingWhenPayloadInvalid(t *testing.T) {
	t.Parallel()

	uc := NewTranscriptViewUseCase(
		stubTranscriptMediaReader{item: domainmedia.Media{ID: 11, Status: domainmedia.StatusTranscribed}},
		stubTranscriptReader{item: transcript.Transcript{MediaID: 11, FullText: "text"}, ok: true},
		stubTriggerEventReader{},
		stubTriggerScreenshotReader{},
		stubSummaryReader{},
		stubTranscriptJobReader{item: job.Job{MediaID: 11, Type: job.TypeTranscribe, Payload: `{"broken":true}`}, ok: true},
		t.TempDir(),
		stubTranscriptAudioDurationReader{},
		t.TempDir(),
		t.TempDir(),
		5*time.Minute,
	)

	result, err := uc.Load(context.Background(), 11)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !result.HasTranscript {
		t.Fatal("HasTranscript = false, want true")
	}
	if result.Settings != nil {
		t.Fatalf("Settings = %#v, want nil for invalid payload", result.Settings)
	}
	if !result.SettingsUnavailable {
		t.Fatal("SettingsUnavailable = false, want true")
	}
}

func TestTranscriptViewUseCase_LoadBuildsRuntimePolicyFromAudioDuration(t *testing.T) {
	t.Parallel()

	audioDir := t.TempDir()

	uc := NewTranscriptViewUseCase(
		stubTranscriptMediaReader{item: domainmedia.Media{
			ID:                 77,
			Status:             domainmedia.StatusFailed,
			ExtractedAudioPath: "2026-04-03/demo.wav",
		}},
		stubTranscriptReader{},
		stubTriggerEventReader{},
		stubTriggerScreenshotReader{},
		stubSummaryReader{},
		stubTranscriptJobReader{item: job.Job{MediaID: 77, Type: job.TypeTranscribe, Payload: mustEncodeTranscriptPayload(t, transcription.Settings{
			Backend:     transcription.BackendFasterWhisper,
			ModelName:   "small",
			Device:      "cpu",
			ComputeType: "int8",
			Language:    "ru",
			BeamSize:    5,
			VADEnabled:  true,
		})}, ok: true},
		t.TempDir(),
		stubTranscriptAudioDurationReader{duration: 60 * time.Minute},
		audioDir,
		t.TempDir(),
		5*time.Minute,
	)

	result, err := uc.Load(context.Background(), 77)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !result.RuntimePolicyReady || result.RuntimePolicy == nil {
		t.Fatal("RuntimePolicy = nil, want computed policy")
	}
	if result.RuntimePolicy.EffectiveTimeout != 9*time.Hour+15*time.Minute {
		t.Fatalf("EffectiveTimeout = %s, want 9h15m", result.RuntimePolicy.EffectiveTimeout)
	}
}

func TestTranscriptViewUseCase_LoadResolvesAudioFallbackSource(t *testing.T) {
	t.Parallel()

	audioDir := t.TempDir()
	audioRelative := "2026-04-03/demo.wav"
	audioPath := filepath.Join(audioDir, filepath.FromSlash(audioRelative))
	if err := os.MkdirAll(filepath.Dir(audioPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(audio dir) error = %v", err)
	}
	if err := os.WriteFile(audioPath, []byte("audio"), 0o644); err != nil {
		t.Fatalf("WriteFile(audio) error = %v", err)
	}

	uc := NewTranscriptViewUseCase(
		stubTranscriptMediaReader{item: domainmedia.Media{
			ID:                 91,
			Status:             domainmedia.StatusTranscribed,
			ExtractedAudioPath: audioRelative,
		}},
		stubTranscriptReader{},
		stubTriggerEventReader{},
		stubTriggerScreenshotReader{},
		stubSummaryReader{},
		stubTranscriptJobReader{},
		t.TempDir(),
		stubTranscriptAudioDurationReader{},
		audioDir,
		t.TempDir(),
		5*time.Minute,
	)

	result, err := uc.Load(context.Background(), 91)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !result.AudioSourceReady {
		t.Fatal("AudioSourceReady = false, want true")
	}
	if result.AudioSourcePath != audioRelative {
		t.Fatalf("AudioSourcePath = %q, want %q", result.AudioSourcePath, audioRelative)
	}
}

type stubTranscriptMediaReader struct {
	item domainmedia.Media
	err  error
}

func (s stubTranscriptMediaReader) GetByID(context.Context, int64) (domainmedia.Media, error) {
	if s.err != nil {
		return domainmedia.Media{}, s.err
	}
	return s.item, nil
}

type stubTranscriptReader struct {
	item transcript.Transcript
	ok   bool
	err  error
}

func (s stubTranscriptReader) GetByMediaID(context.Context, int64) (transcript.Transcript, bool, error) {
	if s.err != nil {
		return transcript.Transcript{}, false, s.err
	}
	return s.item, s.ok, nil
}

type stubTranscriptJobReader struct {
	item  job.Job
	items []job.Job
	ok    bool
	err   error
}

func (s stubTranscriptJobReader) FindLatestByMediaAndType(context.Context, int64, job.Type) (job.Job, bool, error) {
	if s.err != nil {
		return job.Job{}, false, s.err
	}
	return s.item, s.ok, nil
}

func (s stubTranscriptJobReader) ListByMediaID(context.Context, int64) ([]job.Job, error) {
	if s.err != nil {
		return nil, s.err
	}
	if len(s.items) > 0 {
		return append([]job.Job(nil), s.items...), nil
	}
	if s.ok {
		return []job.Job{s.item}, nil
	}
	return nil, nil
}

type stubTriggerEventReader struct {
	items []domaintrigger.Event
	err   error
}

func (s stubTriggerEventReader) ListByMediaID(context.Context, int64) ([]domaintrigger.Event, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.items, nil
}

type stubTriggerScreenshotReader struct {
	items []domaintrigger.Screenshot
	err   error
}

func (s stubTriggerScreenshotReader) ListByMediaID(context.Context, int64) ([]domaintrigger.Screenshot, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.items, nil
}

type stubSummaryReader struct {
	item domainsummary.Summary
	ok   bool
	err  error
}

func (s stubSummaryReader) GetByMediaID(context.Context, int64) (domainsummary.Summary, bool, error) {
	if s.err != nil {
		return domainsummary.Summary{}, false, s.err
	}
	return s.item, s.ok, nil
}

type stubTranscriptAudioDurationReader struct {
	duration time.Duration
	err      error
}

func (s stubTranscriptAudioDurationReader) ReadDuration(string) (time.Duration, error) {
	if s.err != nil {
		return 0, s.err
	}
	return s.duration, nil
}

func mustEncodeTranscriptPayload(t *testing.T, settings transcription.Settings) string {
	t.Helper()

	raw, err := job.EncodeTranscribePayload(job.TranscribePayload{Settings: settings})
	if err != nil {
		t.Fatalf("EncodeTranscribePayload() error = %v", err)
	}

	return raw
}

func TestTranscriptViewUseCase_LoadPropagatesMediaError(t *testing.T) {
	t.Parallel()

	uc := NewTranscriptViewUseCase(
		stubTranscriptMediaReader{err: errors.New("boom")},
		stubTranscriptReader{},
		stubTriggerEventReader{},
		stubTriggerScreenshotReader{},
		stubSummaryReader{},
		stubTranscriptJobReader{},
		t.TempDir(),
		stubTranscriptAudioDurationReader{},
		t.TempDir(),
		t.TempDir(),
		5*time.Minute,
	)

	if _, err := uc.Load(context.Background(), 1); err == nil {
		t.Fatal("Load() error = nil, want wrapped error")
	}
}
