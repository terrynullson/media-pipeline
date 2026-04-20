package handlers_test

import (
	"bytes"
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

func TestRetryJob_RequeuesFailedJob(t *testing.T) {
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
		StoragePath:  "2026-04-14/video.mp4",
		Status:       media.StatusFailed,
		CreatedAtUTC: now,
		UpdatedAtUTC: now,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}

	jobID, err := jobRepo.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeExtractAudio,
		Status:       job.StatusFailed,
		ErrorMessage: "ffmpeg завершился с ошибкой",
		CreatedAtUTC: now,
		UpdatedAtUTC: now,
	})
	if err != nil {
		t.Fatalf("Create(job) error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/media/%d/retry", mediaID), nil)
	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Status string `json:"status"`
		JobID  int64  `json:"jobId"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Status != "requeued" {
		t.Errorf("status = %q, want requeued", payload.Status)
	}
	if payload.JobID != jobID {
		t.Errorf("jobId = %d, want %d", payload.JobID, jobID)
	}

	// Verify the job is now pending in the database.
	j, ok, err := jobRepo.FindLatestByMediaAndType(ctx, mediaID, job.TypeExtractAudio)
	if err != nil {
		t.Fatalf("FindLatestByMediaAndType() error = %v", err)
	}
	if !ok {
		t.Fatal("FindLatestByMediaAndType() ok = false, want true")
	}
	if j.Status != job.StatusPending {
		t.Errorf("job status = %q, want pending", j.Status)
	}
}

func TestRetryJob_NoFailedJobReturns400(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	ctx := context.Background()
	mediaRepo := repositories.NewMediaRepository(app.db)

	now := time.Now().UTC()
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName: "audio.wav",
		StoredName:   "audio.wav",
		Extension:    ".wav",
		MIMEType:     "audio/wav",
		SizeBytes:    512,
		StoragePath:  "2026-04-14/audio.wav",
		Status:       media.StatusUploaded,
		CreatedAtUTC: now,
		UpdatedAtUTC: now,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/media/%d/retry", mediaID), nil)
	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestTriggerPreview_MatchesSegments(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	ctx := context.Background()
	mediaRepo := repositories.NewMediaRepository(app.db)
	transcriptRepo := repositories.NewTranscriptRepository(app.db)

	now := time.Now().UTC()

	// Create two media files with transcripts.
	createMediaWithTranscript := func(name string, segments []transcript.Segment) int64 {
		mediaID, err := mediaRepo.Create(ctx, media.Media{
			OriginalName: name,
			StoredName:   name,
			Extension:    ".mp3",
			MIMEType:     "audio/mpeg",
			SizeBytes:    1024,
			StoragePath:  "2026-04-14/" + name,
			Status:       media.StatusTranscribed,
			CreatedAtUTC: now,
			UpdatedAtUTC: now,
		})
		if err != nil {
			t.Fatalf("Create(media) error = %v", err)
		}
		if err := transcriptRepo.Save(ctx, transcript.Transcript{
			MediaID:      mediaID,
			Language:     "ru",
			FullText:     "text",
			Segments:     segments,
			CreatedAtUTC: now,
			UpdatedAtUTC: now,
		}); err != nil {
			t.Fatalf("Save(transcript) error = %v", err)
		}
		return mediaID
	}

	// media1 has 3 matching segments, media2 has 1 matching segment.
	// "бюджет" must be a standalone word (boundary check in matcher).
	createMediaWithTranscript("meeting1.mp3", []transcript.Segment{
		{StartSec: 0, EndSec: 5, Text: "обсуждение бюджет проекта"},
		{StartSec: 5, EndSec: 10, Text: "утверждение бюджет на следующий год"},
		{StartSec: 10, EndSec: 15, Text: "пересмотр бюджет в третьем квартале"},
		{StartSec: 15, EndSec: 20, Text: "вопросы по маркетингу"},
	})
	createMediaWithTranscript("meeting2.mp3", []transcript.Segment{
		{StartSec: 0, EndSec: 5, Text: "обсуждение плана"},
		{StartSec: 5, EndSec: 10, Text: "финальный бюджет"},
	})
	// media3 has no matches.
	createMediaWithTranscript("meeting3.mp3", []transcript.Segment{
		{StartSec: 0, EndSec: 5, Text: "другая тема без ключевых слов"},
	})

	body, _ := json.Marshal(map[string]string{"pattern": "бюджет", "matchMode": "contains"})
	req := httptest.NewRequest(http.MethodPost, "/api/trigger-rules/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var result struct {
		TotalMatches int `json:"totalMatches"`
		MediaMatches []struct {
			MediaID    int64 `json:"mediaId"`
			MatchCount int   `json:"matchCount"`
		} `json:"mediaMatches"`
		Limited bool `json:"limited"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result.TotalMatches != 4 {
		t.Errorf("totalMatches = %d, want 4", result.TotalMatches)
	}
	if len(result.MediaMatches) != 2 {
		t.Errorf("len(mediaMatches) = %d, want 2", len(result.MediaMatches))
	}
	if result.Limited {
		t.Error("limited = true, want false")
	}
	// Top result should have 3 matches (meeting1).
	if len(result.MediaMatches) > 0 && result.MediaMatches[0].MatchCount != 3 {
		t.Errorf("first mediaMatch.matchCount = %d, want 3", result.MediaMatches[0].MatchCount)
	}
}

func TestTriggerPreview_NoMatches(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	ctx := context.Background()
	mediaRepo := repositories.NewMediaRepository(app.db)
	transcriptRepo := repositories.NewTranscriptRepository(app.db)

	now := time.Now().UTC()
	mediaID, _ := mediaRepo.Create(ctx, media.Media{
		OriginalName: "audio.mp3",
		StoredName:   "audio.mp3",
		Extension:    ".mp3",
		MIMEType:     "audio/mpeg",
		SizeBytes:    1024,
		StoragePath:  "2026-04-14/audio.mp3",
		Status:       media.StatusTranscribed,
		CreatedAtUTC: now,
		UpdatedAtUTC: now,
	})
	_ = transcriptRepo.Save(ctx, transcript.Transcript{
		MediaID:      mediaID,
		Language:     "ru",
		FullText:     "ничего интересного",
		Segments:     []transcript.Segment{{StartSec: 0, EndSec: 5, Text: "ничего интересного"}},
		CreatedAtUTC: now,
		UpdatedAtUTC: now,
	})

	body, _ := json.Marshal(map[string]string{"pattern": "несуществующее", "matchMode": "contains"})
	req := httptest.NewRequest(http.MethodPost, "/api/trigger-rules/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var result struct {
		TotalMatches int `json:"totalMatches"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.TotalMatches != 0 {
		t.Errorf("totalMatches = %d, want 0", result.TotalMatches)
	}
}

func TestBulkDeleteMedia_PartialSuccess(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	ctx := context.Background()
	mediaRepo := repositories.NewMediaRepository(app.db)

	now := time.Now().UTC()
	createMedia := func(name string) int64 {
		id, err := mediaRepo.Create(ctx, media.Media{
			OriginalName: name,
			StoredName:   name,
			Extension:    ".mp3",
			MIMEType:     "audio/mpeg",
			SizeBytes:    512,
			StoragePath:  "2026-04-14/" + name,
			Status:       media.StatusUploaded,
			CreatedAtUTC: now,
			UpdatedAtUTC: now,
		})
		if err != nil {
			t.Fatalf("Create(media) error = %v", err)
		}
		return id
	}

	id1 := createMedia("a.mp3")
	id2 := createMedia("b.mp3")
	id3 := createMedia("c.mp3")
	nonExistentID := int64(99999)

	body, _ := json.Marshal(map[string]any{"ids": []int64{id1, id2, id3, nonExistentID}})
	req := httptest.NewRequest(http.MethodPost, "/api/media/bulk-delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var result struct {
		Deleted []int64 `json:"deleted"`
		Failed  []struct {
			ID    int64  `json:"id"`
			Error string `json:"error"`
		} `json:"failed"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(result.Deleted) != 3 {
		t.Errorf("len(deleted) = %d, want 3", len(result.Deleted))
	}
	if len(result.Failed) != 1 {
		t.Errorf("len(failed) = %d, want 1", len(result.Failed))
	}
	if len(result.Failed) > 0 && result.Failed[0].ID != nonExistentID {
		t.Errorf("failed[0].id = %d, want %d", result.Failed[0].ID, nonExistentID)
	}
}
