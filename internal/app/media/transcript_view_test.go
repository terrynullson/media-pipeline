package mediaapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"media-pipeline/internal/domain/job"
	domainmedia "media-pipeline/internal/domain/media"
	"media-pipeline/internal/domain/transcript"
	"media-pipeline/internal/domain/transcription"
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
		stubTranscriptJobReader{item: job.Job{MediaID: 42, Type: job.TypeTranscribe, Payload: payload}, ok: true},
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
		stubTranscriptJobReader{item: job.Job{MediaID: 11, Type: job.TypeTranscribe, Payload: `{"broken":true}`}, ok: true},
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
	item job.Job
	ok   bool
	err  error
}

func (s stubTranscriptJobReader) FindLatestByMediaAndType(context.Context, int64, job.Type) (job.Job, bool, error) {
	if s.err != nil {
		return job.Job{}, false, s.err
	}
	return s.item, s.ok, nil
}

func TestTranscriptViewUseCase_LoadPropagatesMediaError(t *testing.T) {
	t.Parallel()

	uc := NewTranscriptViewUseCase(
		stubTranscriptMediaReader{err: errors.New("boom")},
		stubTranscriptReader{},
		stubTranscriptJobReader{},
	)

	if _, err := uc.Load(context.Background(), 1); err == nil {
		t.Fatal("Load() error = nil, want wrapped error")
	}
}
