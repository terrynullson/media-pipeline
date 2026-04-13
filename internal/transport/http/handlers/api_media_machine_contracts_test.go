package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/media"
	"media-pipeline/internal/domain/transcript"
	"media-pipeline/internal/infra/db/repositories"
)

func TestAPIMachineStatus_NotFound(t *testing.T) {
	t.Parallel()

	router := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/media/9999/status", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAPIMachineStatus_Queued(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	ctx := context.Background()
	mediaRepo := repositories.NewMediaRepository(app.db)

	now := time.Now().UTC()
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName: "video.mp4",
		StoredName:   "video.mp4",
		Extension:    ".mp4",
		MIMEType:     "video/mp4",
		SizeBytes:    1024,
		StoragePath:  "2026-04-13/video.mp4",
		Status:       media.StatusQueued,
		CreatedAtUTC: now,
		UpdatedAtUTC: now,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/media/%d/status", mediaID), nil)
	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Done       bool   `json:"done"`
		Failed     bool   `json:"failed"`
		StageIndex int    `json:"stageIndex"`
		StageTotal int    `json:"stageTotal"`
		Stage      string `json:"stage"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Done {
		t.Errorf("done = true, want false")
	}
	if payload.Failed {
		t.Errorf("failed = true, want false")
	}
	if payload.Stage != "queued" {
		t.Errorf("stage = %q, want queued", payload.Stage)
	}
	if payload.StageTotal != 6 {
		t.Errorf("stageTotal = %d, want 6 (video)", payload.StageTotal)
	}
}

func TestAPIMachineStatus_AudioOnly_Queued(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	ctx := context.Background()
	mediaRepo := repositories.NewMediaRepository(app.db)

	now := time.Now().UTC()
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName: "audio.mp3",
		StoredName:   "audio.mp3",
		Extension:    ".mp3",
		MIMEType:     "audio/mpeg",
		SizeBytes:    512,
		StoragePath:  "2026-04-13/audio.mp3",
		Status:       media.StatusQueued,
		CreatedAtUTC: now,
		UpdatedAtUTC: now,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/media/%d/status", mediaID), nil)
	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Done       bool `json:"done"`
		StageTotal int  `json:"stageTotal"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Done {
		t.Errorf("done = true, want false")
	}
	if payload.StageTotal != 4 {
		t.Errorf("stageTotal = %d, want 4 (audio-only)", payload.StageTotal)
	}
}

func TestAPIMachineStatus_FailedExtractAudio(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	ctx := context.Background()
	mediaRepo := repositories.NewMediaRepository(app.db)
	jobRepo := repositories.NewJobRepository(app.db)

	now := time.Now().UTC()
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName: "video.mp4",
		StoredName:   "video.mp4",
		Extension:    ".mp4",
		MIMEType:     "video/mp4",
		SizeBytes:    1024,
		StoragePath:  "2026-04-13/video.mp4",
		Status:       media.StatusFailed,
		CreatedAtUTC: now,
		UpdatedAtUTC: now,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}
	if _, err := jobRepo.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeExtractAudio,
		Status:       job.StatusFailed,
		ErrorMessage: "ffmpeg not found",
		CreatedAtUTC: now,
		UpdatedAtUTC: now,
	}); err != nil {
		t.Fatalf("Create(job) error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/media/%d/status", mediaID), nil)
	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Done   bool   `json:"done"`
		Failed bool   `json:"failed"`
		Error  string `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.Done {
		t.Errorf("done = false, want true")
	}
	if !payload.Failed {
		t.Errorf("failed = false, want true")
	}
	if payload.Error != "ffmpeg not found" {
		t.Errorf("error = %q, want %q", payload.Error, "ffmpeg not found")
	}
}

func TestAPIMachineStatus_FailedAnalyzeTriggers(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	ctx := context.Background()
	mediaRepo := repositories.NewMediaRepository(app.db)
	jobRepo := repositories.NewJobRepository(app.db)

	now := time.Now().UTC()
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName: "video.mp4",
		StoredName:   "video.mp4",
		Extension:    ".mp4",
		MIMEType:     "video/mp4",
		SizeBytes:    1024,
		StoragePath:  "2026-04-13/video.mp4",
		Status:       media.StatusFailed,
		CreatedAtUTC: now,
		UpdatedAtUTC: now,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}
	if _, err := jobRepo.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeAnalyzeTriggers,
		Status:       job.StatusFailed,
		ErrorMessage: "analyze error",
		CreatedAtUTC: now,
		UpdatedAtUTC: now,
	}); err != nil {
		t.Fatalf("Create(analyze job) error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/media/%d/status", mediaID), nil)
	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Done   bool   `json:"done"`
		Failed bool   `json:"failed"`
		Error  string `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.Done {
		t.Errorf("done = false, want true")
	}
	if !payload.Failed {
		t.Errorf("failed = false, want true")
	}
	if payload.Error != "analyze error" {
		t.Errorf("error = %q, want %q", payload.Error, "analyze error")
	}
}

func TestAPIMachineStatus_FailedExtractScreenshots(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	ctx := context.Background()
	mediaRepo := repositories.NewMediaRepository(app.db)
	jobRepo := repositories.NewJobRepository(app.db)

	now := time.Now().UTC()
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName: "video.mp4",
		StoredName:   "video.mp4",
		Extension:    ".mp4",
		MIMEType:     "video/mp4",
		SizeBytes:    1024,
		StoragePath:  "2026-04-13/video.mp4",
		Status:       media.StatusFailed,
		CreatedAtUTC: now,
		UpdatedAtUTC: now,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}
	if _, err := jobRepo.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeExtractScreenshots,
		Status:       job.StatusFailed,
		ErrorMessage: "screenshot error",
		CreatedAtUTC: now,
		UpdatedAtUTC: now,
	}); err != nil {
		t.Fatalf("Create(screenshot job) error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/media/%d/status", mediaID), nil)
	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Done   bool   `json:"done"`
		Failed bool   `json:"failed"`
		Error  string `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.Done {
		t.Errorf("done = false, want true")
	}
	if !payload.Failed {
		t.Errorf("failed = false, want true")
	}
	if payload.Error != "screenshot error" {
		t.Errorf("error = %q, want %q", payload.Error, "screenshot error")
	}
}

func TestAPIMachineStatus_Done(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	ctx := context.Background()
	mediaRepo := repositories.NewMediaRepository(app.db)
	jobRepo := repositories.NewJobRepository(app.db)
	transcriptRepo := repositories.NewTranscriptRepository(app.db)

	now := time.Now().UTC()
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName: "video.mp4",
		StoredName:   "video.mp4",
		Extension:    ".mp4",
		MIMEType:     "video/mp4",
		SizeBytes:    1024,
		StoragePath:  "2026-04-13/video.mp4",
		Status:       media.StatusTranscribed,
		CreatedAtUTC: now,
		UpdatedAtUTC: now,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}
	for _, jType := range []job.Type{job.TypeExtractAudio, job.TypeTranscribe, job.TypeAnalyzeTriggers, job.TypeExtractScreenshots} {
		if _, err := jobRepo.Create(ctx, job.Job{
			MediaID:      mediaID,
			Type:         jType,
			Status:       job.StatusDone,
			CreatedAtUTC: now,
			UpdatedAtUTC: now,
		}); err != nil {
			t.Fatalf("Create(job %s) error = %v", jType, err)
		}
	}
	if err := transcriptRepo.Save(ctx, transcript.Transcript{
		MediaID:  mediaID,
		Language: "en",
		FullText: "Hello world.",
	}); err != nil {
		t.Fatalf("Save(transcript) error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/media/%d/status", mediaID), nil)
	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Done       bool   `json:"done"`
		Failed     bool   `json:"failed"`
		StageIndex int    `json:"stageIndex"`
		StageTotal int    `json:"stageTotal"`
		Stage      string `json:"stage"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.Done {
		t.Errorf("done = false, want true")
	}
	if payload.Failed {
		t.Errorf("failed = true, want false")
	}
	if payload.Stage != "completed" {
		t.Errorf("stage = %q, want completed", payload.Stage)
	}
	if payload.StageIndex != payload.StageTotal {
		t.Errorf("stageIndex = %d, stageTotal = %d, want equal", payload.StageIndex, payload.StageTotal)
	}
}

func TestAPIMachineResult_NotReady(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	ctx := context.Background()
	mediaRepo := repositories.NewMediaRepository(app.db)

	now := time.Now().UTC()
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName: "video.mp4",
		StoredName:   "video.mp4",
		Extension:    ".mp4",
		MIMEType:     "video/mp4",
		SizeBytes:    1024,
		StoragePath:  "2026-04-13/video.mp4",
		Status:       media.StatusQueued,
		CreatedAtUTC: now,
		UpdatedAtUTC: now,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/media/%d/result", mediaID), nil)
	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestAPIMachineResult_Ready(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	ctx := context.Background()
	mediaRepo := repositories.NewMediaRepository(app.db)
	transcriptRepo := repositories.NewTranscriptRepository(app.db)

	now := time.Now().UTC()
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName: "audio.mp3",
		StoredName:   "audio.mp3",
		Extension:    ".mp3",
		MIMEType:     "audio/mpeg",
		SizeBytes:    512,
		StoragePath:  "2026-04-13/audio.mp3",
		Status:       media.StatusTranscribed,
		CreatedAtUTC: now,
		UpdatedAtUTC: now,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}
	if err := transcriptRepo.Save(ctx, transcript.Transcript{
		MediaID:  mediaID,
		Language: "ru",
		FullText: "Привет мир.",
	}); err != nil {
		t.Fatalf("Save(transcript) error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/media/%d/result", mediaID), nil)
	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		MediaID    int64  `json:"mediaId"`
		Name       string `json:"name"`
		Transcript string `json:"transcript"`
		Language   string `json:"language"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.MediaID != mediaID {
		t.Errorf("mediaId = %d, want %d", payload.MediaID, mediaID)
	}
	if payload.Name != "audio.mp3" {
		t.Errorf("name = %q, want audio.mp3", payload.Name)
	}
	if payload.Transcript != "Привет мир." {
		t.Errorf("transcript = %q, want Привет мир.", payload.Transcript)
	}
	if payload.Language != "ru" {
		t.Errorf("language = %q, want ru", payload.Language)
	}
}

func TestAPIMachineResult_NotFound(t *testing.T) {
	t.Parallel()

	router := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/media/9999/result", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
