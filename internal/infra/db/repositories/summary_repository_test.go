package repositories

import (
	"context"
	"testing"
	"time"

	"media-pipeline/internal/domain/media"
	domainsummary "media-pipeline/internal/domain/summary"
)

func TestSummaryRepository_SaveAndGetByMediaID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	mediaRepo := NewMediaRepository(sqlDB)
	repo := NewSummaryRepository(sqlDB)

	nowUTC := time.Date(2026, 4, 3, 20, 0, 0, 0, time.UTC)
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName: "summary.wav",
		StoredName:   "summary.wav",
		Extension:    ".wav",
		MIMEType:     "audio/wav",
		SizeBytes:    1024,
		StoragePath:  "2026-04-03/summary.wav",
		Status:       media.StatusTranscribed,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}

	err = repo.Save(ctx, domainsummary.Summary{
		MediaID:      mediaID,
		SummaryText:  "Короткое саммари разговора.",
		Highlights:   []string{"Первый тезис.", "Второй тезис."},
		Provider:     "simple-summary-v1",
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	item, ok, err := repo.GetByMediaID(ctx, mediaID)
	if err != nil {
		t.Fatalf("GetByMediaID() error = %v", err)
	}
	if !ok {
		t.Fatal("GetByMediaID() ok = false, want true")
	}
	if item.SummaryText != "Короткое саммари разговора." {
		t.Fatalf("SummaryText = %q, want persisted text", item.SummaryText)
	}
	if len(item.Highlights) != 2 || item.Highlights[0] != "Первый тезис." {
		t.Fatalf("Highlights = %#v, want persisted highlights", item.Highlights)
	}
	if item.Provider != "simple-summary-v1" {
		t.Fatalf("Provider = %q, want simple-summary-v1", item.Provider)
	}
}

func TestSummaryRepository_SaveUpdatesExistingSummary(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	mediaRepo := NewMediaRepository(sqlDB)
	repo := NewSummaryRepository(sqlDB)

	nowUTC := time.Date(2026, 4, 3, 20, 30, 0, 0, time.UTC)
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName: "summary-update.wav",
		StoredName:   "summary-update.wav",
		Extension:    ".wav",
		MIMEType:     "audio/wav",
		SizeBytes:    1024,
		StoragePath:  "2026-04-03/summary-update.wav",
		Status:       media.StatusTranscribed,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}

	if err := repo.Save(ctx, domainsummary.Summary{
		MediaID:      mediaID,
		SummaryText:  "Первая версия.",
		Highlights:   []string{"Старый тезис."},
		Provider:     "simple-summary-v1",
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	}); err != nil {
		t.Fatalf("Save(first) error = %v", err)
	}

	if err := repo.Save(ctx, domainsummary.Summary{
		MediaID:      mediaID,
		SummaryText:  "Новая версия.",
		Highlights:   []string{"Новый тезис."},
		Provider:     "simple-summary-v1",
		CreatedAtUTC: nowUTC.Add(time.Minute),
		UpdatedAtUTC: nowUTC.Add(time.Minute),
	}); err != nil {
		t.Fatalf("Save(second) error = %v", err)
	}

	item, ok, err := repo.GetByMediaID(ctx, mediaID)
	if err != nil {
		t.Fatalf("GetByMediaID() error = %v", err)
	}
	if !ok {
		t.Fatal("GetByMediaID() ok = false, want true")
	}
	if item.SummaryText != "Новая версия." {
		t.Fatalf("SummaryText = %q, want updated text", item.SummaryText)
	}
	if len(item.Highlights) != 1 || item.Highlights[0] != "Новый тезис." {
		t.Fatalf("Highlights = %#v, want updated highlights", item.Highlights)
	}
}
