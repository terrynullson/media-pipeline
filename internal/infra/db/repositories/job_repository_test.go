package repositories

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/media"
	"media-pipeline/internal/infra/db"
	infraRuntime "media-pipeline/internal/infra/runtime"
)

func TestJobRepository_ClaimNextPendingAndMarkDone(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	mediaRepo := NewMediaRepository(sqlDB)
	jobRepo := NewJobRepository(sqlDB)

	mediaID := createTestMedia(t, ctx, mediaRepo)
	nowUTC := time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC)

	if _, err := jobRepo.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeExtractAudio,
		Payload:      `{"example":true}`,
		Status:       job.StatusPending,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	}); err != nil {
		t.Fatalf("Create(job) error = %v", err)
	}

	claimedJob, ok, err := jobRepo.ClaimNextPending(ctx, job.TypeExtractAudio, nowUTC.Add(time.Minute))
	if err != nil {
		t.Fatalf("ClaimNextPending() error = %v", err)
	}
	if !ok {
		t.Fatal("ClaimNextPending() ok = false, want true")
	}
	if claimedJob.Status != job.StatusRunning {
		t.Fatalf("claimed status = %q, want %q", claimedJob.Status, job.StatusRunning)
	}
	if claimedJob.Payload != `{"example":true}` {
		t.Fatalf("claimed payload = %q, want persisted payload", claimedJob.Payload)
	}

	if err := jobRepo.MarkDone(ctx, claimedJob.ID, nowUTC.Add(2*time.Minute)); err != nil {
		t.Fatalf("MarkDone() error = %v", err)
	}

	var status string
	var attempts int
	var errorMessage string
	if err := sqlDB.QueryRowContext(ctx, "SELECT status, attempts, error_message FROM jobs WHERE id = ?", claimedJob.ID).
		Scan(&status, &attempts, &errorMessage); err != nil {
		t.Fatalf("QueryRow(status) error = %v", err)
	}
	if status != string(job.StatusDone) {
		t.Fatalf("status = %q, want %q", status, job.StatusDone)
	}
	if attempts != 0 {
		t.Fatalf("attempts = %d, want 0", attempts)
	}
	if errorMessage != "" {
		t.Fatalf("error_message = %q, want empty", errorMessage)
	}
}

func TestJobRepository_MarkFailedIncrementsAttempts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	mediaRepo := NewMediaRepository(sqlDB)
	jobRepo := NewJobRepository(sqlDB)

	mediaID := createTestMedia(t, ctx, mediaRepo)
	nowUTC := time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC)

	jobID, err := jobRepo.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeExtractAudio,
		Status:       job.StatusPending,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	})
	if err != nil {
		t.Fatalf("Create(job) error = %v", err)
	}

	if err := jobRepo.MarkFailed(ctx, jobID, "ffmpeg failed", nowUTC.Add(time.Minute)); err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}

	var status string
	var attempts int
	var errorMessage string
	if err := sqlDB.QueryRowContext(ctx, "SELECT status, attempts, error_message FROM jobs WHERE id = ?", jobID).
		Scan(&status, &attempts, &errorMessage); err != nil {
		t.Fatalf("QueryRow(status) error = %v", err)
	}
	if status != string(job.StatusFailed) {
		t.Fatalf("status = %q, want %q", status, job.StatusFailed)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
	if errorMessage != "ffmpeg failed" {
		t.Fatalf("error_message = %q, want %q", errorMessage, "ffmpeg failed")
	}
}

func TestJobRepository_RequeueMovesRunningBackToPending(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	mediaRepo := NewMediaRepository(sqlDB)
	jobRepo := NewJobRepository(sqlDB)

	mediaID := createTestMedia(t, ctx, mediaRepo)
	nowUTC := time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC)

	jobID, err := jobRepo.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeExtractAudio,
		Status:       job.StatusRunning,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	})
	if err != nil {
		t.Fatalf("Create(job) error = %v", err)
	}

	jobs, err := jobRepo.ListByStatus(ctx, job.TypeExtractAudio, job.StatusRunning)
	if err != nil {
		t.Fatalf("ListByStatus() error = %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != jobID {
		t.Fatalf("ListByStatus() jobs = %#v, want one running job %d", jobs, jobID)
	}

	if err := jobRepo.Requeue(ctx, jobID, "worker restarted before job completion", nowUTC.Add(time.Minute)); err != nil {
		t.Fatalf("Requeue() error = %v", err)
	}

	var status string
	var attempts int
	var errorMessage string
	if err := sqlDB.QueryRowContext(ctx, "SELECT status, attempts, error_message FROM jobs WHERE id = ?", jobID).
		Scan(&status, &attempts, &errorMessage); err != nil {
		t.Fatalf("QueryRow(status) error = %v", err)
	}
	if status != string(job.StatusPending) {
		t.Fatalf("status = %q, want %q", status, job.StatusPending)
	}
	if attempts != 0 {
		t.Fatalf("attempts = %d, want 0", attempts)
	}
	if errorMessage != "worker restarted before job completion" {
		t.Fatalf("error_message = %q, want recovery message", errorMessage)
	}
}

func TestJobRepository_ExistsActiveOrDone(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	mediaRepo := NewMediaRepository(sqlDB)
	jobRepo := NewJobRepository(sqlDB)

	mediaID := createTestMedia(t, ctx, mediaRepo)
	nowUTC := time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC)

	if _, err := jobRepo.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeTranscribe,
		Status:       job.StatusPending,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	}); err != nil {
		t.Fatalf("Create(job) error = %v", err)
	}

	exists, err := jobRepo.ExistsActiveOrDone(ctx, mediaID, job.TypeTranscribe)
	if err != nil {
		t.Fatalf("ExistsActiveOrDone() error = %v", err)
	}
	if !exists {
		t.Fatal("ExistsActiveOrDone() = false, want true")
	}
}

func TestJobRepository_FindLatestByMediaAndTypeReturnsNewest(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	mediaRepo := NewMediaRepository(sqlDB)
	jobRepo := NewJobRepository(sqlDB)

	mediaID := createTestMedia(t, ctx, mediaRepo)
	nowUTC := time.Date(2026, 4, 3, 15, 0, 0, 0, time.UTC)

	if _, err := jobRepo.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeTranscribe,
		Payload:      `{"settings":{"model_name":"tiny"}}`,
		Status:       job.StatusPending,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	}); err != nil {
		t.Fatalf("Create(first job) error = %v", err)
	}
	if _, err := jobRepo.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeTranscribe,
		Payload:      `{"settings":{"model_name":"small"}}`,
		Status:       job.StatusDone,
		CreatedAtUTC: nowUTC.Add(time.Minute),
		UpdatedAtUTC: nowUTC.Add(time.Minute),
	}); err != nil {
		t.Fatalf("Create(second job) error = %v", err)
	}

	item, ok, err := jobRepo.FindLatestByMediaAndType(ctx, mediaID, job.TypeTranscribe)
	if err != nil {
		t.Fatalf("FindLatestByMediaAndType() error = %v", err)
	}
	if !ok {
		t.Fatal("FindLatestByMediaAndType() ok = false, want true")
	}
	if item.Payload != `{"settings":{"model_name":"small"}}` {
		t.Fatalf("Payload = %q, want newest payload", item.Payload)
	}
}

func TestJobRepository_ListByMediaIDReturnsNewestFirst(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	mediaRepo := NewMediaRepository(sqlDB)
	jobRepo := NewJobRepository(sqlDB)

	mediaID := createTestMedia(t, ctx, mediaRepo)
	nowUTC := time.Date(2026, 4, 3, 16, 0, 0, 0, time.UTC)

	firstID, err := jobRepo.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeExtractAudio,
		Status:       job.StatusDone,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	})
	if err != nil {
		t.Fatalf("Create(first job) error = %v", err)
	}
	secondID, err := jobRepo.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeTranscribe,
		Status:       job.StatusFailed,
		ErrorMessage: "transcribe failed",
		CreatedAtUTC: nowUTC.Add(time.Minute),
		UpdatedAtUTC: nowUTC.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("Create(second job) error = %v", err)
	}

	items, err := jobRepo.ListByMediaID(ctx, mediaID)
	if err != nil {
		t.Fatalf("ListByMediaID() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("ListByMediaID() len = %d, want 2", len(items))
	}
	if items[0].ID != secondID {
		t.Fatalf("items[0].ID = %d, want %d", items[0].ID, secondID)
	}
	if items[1].ID != firstID {
		t.Fatalf("items[1].ID = %d, want %d", items[1].ID, firstID)
	}
	if items[0].ErrorMessage != "transcribe failed" {
		t.Fatalf("items[0].ErrorMessage = %q, want persisted error", items[0].ErrorMessage)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "app.db")

	sqlDB, err := db.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}

	migrationsPath, err := infraRuntime.ResolvePath("internal/infra/db/migrations")
	if err != nil {
		t.Fatalf("ResolvePath(migrations) error = %v", err)
	}
	if err := db.RunMigrations(sqlDB, migrationsPath); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	return sqlDB
}

func createTestMedia(t *testing.T, ctx context.Context, mediaRepo *MediaRepository) int64 {
	t.Helper()

	nowUTC := time.Date(2026, 4, 3, 9, 0, 0, 0, time.UTC)
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName: "sample.mp4",
		StoredName:   "sample.mp4",
		Extension:    ".mp4",
		MIMEType:     "video/mp4",
		SizeBytes:    1024,
		StoragePath:  "2026-04-03/sample.mp4",
		Status:       media.StatusUploaded,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}

	return mediaID
}
