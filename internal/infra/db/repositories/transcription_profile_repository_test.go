package repositories

import (
	"context"
	"testing"
	"time"

	"media-pipeline/internal/domain/transcription"
)

func TestTranscriptionProfileRepository_SaveAndGetDefault(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	repo := NewTranscriptionProfileRepository(sqlDB)
	nowUTC := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)

	saved, err := repo.Save(ctx, transcription.Profile{
		Backend:      transcription.BackendFasterWhisper,
		ModelName:    "tiny",
		Device:       "cpu",
		ComputeType:  "int8",
		Language:     "ru",
		BeamSize:     5,
		VADEnabled:   true,
		IsDefault:    true,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if saved.ID == 0 {
		t.Fatal("Save() id = 0, want inserted id")
	}

	current, ok, err := repo.GetDefault(ctx)
	if err != nil {
		t.Fatalf("GetDefault() error = %v", err)
	}
	if !ok {
		t.Fatal("GetDefault() ok = false, want true")
	}
	if current.ModelName != "tiny" || current.Device != "cpu" || !current.VADEnabled {
		t.Fatalf("GetDefault() profile = %#v, want saved values", current)
	}
}

func TestTranscriptionProfileRepository_SaveReplacesDefault(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	repo := NewTranscriptionProfileRepository(sqlDB)
	nowUTC := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)

	first, err := repo.Save(ctx, transcription.Profile{
		Backend:      transcription.BackendFasterWhisper,
		ModelName:    "tiny",
		Device:       "cpu",
		ComputeType:  "int8",
		Language:     "ru",
		BeamSize:     5,
		VADEnabled:   true,
		IsDefault:    true,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	})
	if err != nil {
		t.Fatalf("Save(first) error = %v", err)
	}

	if _, err := repo.Save(ctx, transcription.Profile{
		ID:           first.ID,
		Backend:      transcription.BackendFasterWhisper,
		ModelName:    "small",
		Device:       "cuda",
		ComputeType:  "float16",
		Language:     "en",
		BeamSize:     3,
		VADEnabled:   false,
		IsDefault:    true,
		CreatedAtUTC: first.CreatedAtUTC,
		UpdatedAtUTC: nowUTC.Add(time.Minute),
	}); err != nil {
		t.Fatalf("Save(update) error = %v", err)
	}

	current, ok, err := repo.GetDefault(ctx)
	if err != nil {
		t.Fatalf("GetDefault() error = %v", err)
	}
	if !ok {
		t.Fatal("GetDefault() ok = false, want true")
	}
	if current.ModelName != "small" || current.Device != "cuda" || current.VADEnabled {
		t.Fatalf("GetDefault() profile = %#v, want updated values", current)
	}
}
