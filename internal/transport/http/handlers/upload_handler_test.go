package handlers_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"media-pipeline/internal/app/command"
	mediaapp "media-pipeline/internal/app/media"
	transcriptionapp "media-pipeline/internal/app/transcription"
	triggerapp "media-pipeline/internal/app/trigger"
	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/media"
	domainsummary "media-pipeline/internal/domain/summary"
	"media-pipeline/internal/domain/transcript"
	domaintranscription "media-pipeline/internal/domain/transcription"
	domaintrigger "media-pipeline/internal/domain/trigger"
	"media-pipeline/internal/infra/db"
	"media-pipeline/internal/infra/db/repositories"
	infraRuntime "media-pipeline/internal/infra/runtime"
	"media-pipeline/internal/infra/storage"
	httptransport "media-pipeline/internal/transport/http"
	"media-pipeline/internal/transport/http/handlers"
)

func TestUploadHandler_UploadHappyPath(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	router := app.router

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("media", "sample.wav")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write(testWAVBytes()); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() multipart error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("upload status = %d, want %d, body = %s", rec.Code, http.StatusSeeOther, rec.Body.String())
	}
	if location := rec.Header().Get("Location"); location != "/?status=uploaded" {
		t.Fatalf("upload redirect = %q, want %q", location, "/?status=uploaded")
	}

	files, err := os.ReadDir(app.uploadDir)
	if err != nil {
		t.Fatalf("ReadDir(uploadDir) error = %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("upload dir entries = %d, want 1 dated directory", len(files))
	}
}

func TestUploadHandler_InvalidUpload(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	router := app.router

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("media", "fake.mp4")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write([]byte("not media")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() multipart error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("upload status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "Файл не похож на аудио или видео.") {
		t.Fatalf("response body = %q, want content type validation message", rec.Body.String())
	}
}

func TestUploadHandler_UploadHappyPathJSON(t *testing.T) {
	t.Parallel()

	router := newTestRouter(t)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("media", "sample.wav")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write(testWAVBytes()); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() multipart error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, want %d, body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var payload struct {
		Status  string `json:"status"`
		MediaID int64  `json:"mediaId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Status != "uploaded" {
		t.Fatalf("status = %q, want uploaded", payload.Status)
	}
	if payload.MediaID == 0 {
		t.Fatalf("mediaId = %d, want non-zero", payload.MediaID)
	}
}

func TestUploadHandler_UploadPersistsRuntimeSnapshot(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	router := app.router

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("hardware_concurrency", "8"); err != nil {
		t.Fatalf("WriteField(hardware_concurrency) error = %v", err)
	}
	if err := writer.WriteField("device_memory_gb", "16"); err != nil {
		t.Fatalf("WriteField(device_memory_gb) error = %v", err)
	}
	part, err := writer.CreateFormFile("media", "sample.wav")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write(testWAVBytes()); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() multipart error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "snapshot-agent")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("upload status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	mediaRepo := repositories.NewMediaRepository(app.db)
	items, err := mediaRepo.ListRecent(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListRecent() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("ListRecent() len = %d, want 1", len(items))
	}
	if !strings.Contains(items[0].RuntimeSnapshotJSON, "snapshot-agent") {
		t.Fatalf("RuntimeSnapshotJSON = %q, want user agent", items[0].RuntimeSnapshotJSON)
	}
}

func TestUploadHandler_MediaStatuses(t *testing.T) {
	t.Parallel()

	router := newTestRouter(t)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("media", "sample.wav")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write(testWAVBytes()); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() multipart error = %v", err)
	}

	uploadReq := httptest.NewRequest(http.MethodPost, "/upload", &body)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRec := httptest.NewRecorder()
	router.ServeHTTP(uploadRec, uploadReq)

	req := httptest.NewRequest(http.MethodGet, "/media/statuses", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status endpoint code = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Items []struct {
			ID           int64  `json:"ID"`
			Status       string `json:"Status"`
			StatusLabel  string `json:"StatusLabel"`
			StageLabel   string `json:"StageLabel"`
			FailedStage  string `json:"FailedStage"`
			ErrorSummary string `json:"ErrorSummary"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(payload.Items))
	}
	if payload.Items[0].Status != "queued" {
		t.Fatalf("status = %q, want queued", payload.Items[0].Status)
	}
	if payload.Items[0].StatusLabel != "В очереди" {
		t.Fatalf("status label = %q, want В очереди", payload.Items[0].StatusLabel)
	}
	if payload.Items[0].StageLabel != "Ждёт запуска основной обработки" {
		t.Fatalf("stage label = %q, want Ждёт запуска основной обработки", payload.Items[0].StageLabel)
	}
	if payload.Items[0].FailedStage != "" || payload.Items[0].ErrorSummary != "" {
		t.Fatalf("unexpected failure details: %#v", payload.Items[0])
	}
}

func TestUploadHandler_TranscriptPage(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	ctx := context.Background()
	mediaRepo := repositories.NewMediaRepository(app.db)
	jobRepo := repositories.NewJobRepository(app.db)
	transcriptRepo := repositories.NewTranscriptRepository(app.db)
	triggerEventRepo := repositories.NewTriggerEventRepository(app.db)
	triggerRuleRepo := repositories.NewTriggerRuleRepository(app.db)
	triggerScreenshotRepo := repositories.NewTriggerScreenshotRepository(app.db)
	summaryRepo := repositories.NewSummaryRepository(app.db)

	nowUTC := time.Date(2026, 4, 3, 16, 0, 0, 0, time.UTC)
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName:        "timeline.mp4",
		StoredName:          "timeline.mp4",
		Extension:           ".mp4",
		MIMEType:            "video/mp4",
		SizeBytes:           2048,
		StoragePath:         "2026-04-03/timeline.mp4",
		ExtractedAudioPath:  "2026-04-03/timeline.wav",
		RuntimeSnapshotJSON: `{"request_ip":"127.0.0.1","user_agent":"test-agent","hardware_concurrency":8}`,
		Status:              media.StatusTranscribed,
		CreatedAtUTC:        nowUTC,
		UpdatedAtUTC:        nowUTC,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}
	createUploadedMediaFile(t, app.uploadDir, "2026-04-03/timeline.mp4", []byte("video"))
	createUploadedMediaFile(t, app.audioDir, "2026-04-03/timeline.wav", []byte("audio"))

	settingsPayload, err := job.EncodeTranscribePayload(job.TranscribePayload{
		Settings: domaintranscription.Settings{
			Backend:     domaintranscription.BackendFasterWhisper,
			ModelName:   "small",
			Device:      "cpu",
			ComputeType: "int8",
			Language:    "ru",
			BeamSize:    3,
			VADEnabled:  true,
		},
	})
	if err != nil {
		t.Fatalf("EncodeTranscribePayload() error = %v", err)
	}
	if _, err := jobRepo.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeTranscribe,
		Payload:      settingsPayload,
		Status:       job.StatusDone,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	}); err != nil {
		t.Fatalf("Create(job) error = %v", err)
	}
	if _, err := jobRepo.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeAnalyzeTriggers,
		Status:       job.StatusDone,
		CreatedAtUTC: nowUTC.Add(time.Minute),
		UpdatedAtUTC: nowUTC.Add(time.Minute),
	}); err != nil {
		t.Fatalf("Create(analyze job) error = %v", err)
	}
	if _, err := jobRepo.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeExtractScreenshots,
		Status:       job.StatusDone,
		CreatedAtUTC: nowUTC.Add(2 * time.Minute),
		UpdatedAtUTC: nowUTC.Add(2 * time.Minute),
	}); err != nil {
		t.Fatalf("Create(screenshot job) error = %v", err)
	}

	confidence := 0.87
	if err := transcriptRepo.Save(ctx, transcript.Transcript{
		MediaID:      mediaID,
		Language:     "ru",
		FullText:     "Hello world. Nice to meet you.",
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
		Segments: []transcript.Segment{
			{StartSec: 0, EndSec: 1.4, Text: "Hello world.", Confidence: &confidence},
			{StartSec: 2.1, EndSec: 3.8, Text: "Nice to meet you."},
		},
	}); err != nil {
		t.Fatalf("Save(transcript) error = %v", err)
	}
	if err := mediaRepo.MarkTranscribed(ctx, mediaID, "Hello world. Nice to meet you.", nowUTC); err != nil {
		t.Fatalf("MarkTranscribed() error = %v", err)
	}
	previewRelative := filepath.ToSlash(filepath.Join("2026-04-03", "timeline_preview.mp4"))
	createUploadedMediaFile(t, app.previewDir, previewRelative, []byte("preview"))
	if err := mediaRepo.MarkPreviewReady(ctx, mediaID, previewRelative, int64(len("preview")), "video/mp4", nowUTC, nowUTC); err != nil {
		t.Fatalf("MarkPreviewReady() error = %v", err)
	}
	transcriptItem, ok, err := transcriptRepo.GetByMediaID(ctx, mediaID)
	if err != nil {
		t.Fatalf("GetByMediaID(transcript) error = %v", err)
	}
	if !ok {
		t.Fatal("GetByMediaID(transcript) ok = false, want true")
	}
	triggerRules, err := triggerRuleRepo.List(ctx)
	if err != nil {
		t.Fatalf("List(trigger rules) error = %v", err)
	}
	if len(triggerRules) == 0 {
		t.Fatal("trigger rules are empty, want seeded rules")
	}
	transcriptID := transcriptItem.ID
	if err := triggerEventRepo.ReplaceForMedia(ctx, mediaID, &transcriptID, []domaintrigger.Event{
		{
			MediaID:      mediaID,
			TranscriptID: &transcriptID,
			RuleID:       triggerRules[0].ID,
			Category:     "billing",
			MatchedText:  "refund",
			SegmentIndex: 1,
			StartSec:     2.1,
			EndSec:       3.8,
			SegmentText:  "Nice to meet you. Please refund this order.",
			ContextText:  "Hello world. Nice to meet you. Please refund this order.",
			CreatedAtUTC: nowUTC,
		},
	}); err != nil {
		t.Fatalf("ReplaceForMedia(trigger events) error = %v", err)
	}
	triggerEvents, err := triggerEventRepo.ListByMediaID(ctx, mediaID)
	if err != nil {
		t.Fatalf("ListByMediaID(trigger events) error = %v", err)
	}
	if len(triggerEvents) != 1 {
		t.Fatalf("trigger events = %d, want 1", len(triggerEvents))
	}
	screenshotRelative := filepath.ToSlash(filepath.Join("2026-04-03", "media_1_trigger_1_2100ms.jpg"))
	screenshotPath := filepath.Join(app.screenshotsDir, filepath.FromSlash(screenshotRelative))
	if err := os.MkdirAll(filepath.Dir(screenshotPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(screenshot dir) error = %v", err)
	}
	if err := os.WriteFile(screenshotPath, pngPixelBytes(), 0o644); err != nil {
		t.Fatalf("WriteFile(screenshot) error = %v", err)
	}
	if err := triggerScreenshotRepo.ReplaceForMedia(ctx, mediaID, []domaintrigger.Screenshot{
		{
			MediaID:        mediaID,
			TriggerEventID: triggerEvents[0].ID,
			TimestampSec:   2.1,
			ImagePath:      screenshotRelative,
			Width:          1,
			Height:         1,
			CreatedAtUTC:   nowUTC,
		},
	}); err != nil {
		t.Fatalf("ReplaceForMedia(trigger screenshots) error = %v", err)
	}
	if err := summaryRepo.Save(ctx, domainsummary.Summary{
		MediaID:      mediaID,
		SummaryText:  "Короткое саммари по разговору.",
		Highlights:   []string{"Обсудили заказ.", "Попросили refund."},
		Provider:     "simple-summary-v1",
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	}); err != nil {
		t.Fatalf("Save(summary) error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/media/"+strconv.FormatInt(mediaID, 10)+"/transcript", nil)
	rec := httptest.NewRecorder()

	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("transcript page status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"timeline.mp4",
		"Результат и статус пайплайна для этого файла.",
		"Полный текст",
		"Таймлайн",
		"Совпадения по триггерам",
		"Саммари",
		"Сделать заново",
		"Короткое саммари по разговору.",
		"refund",
		"billing",
		"/media-screenshots/2026-04-03/media_1_trigger_1_2100ms.jpg",
		"/media-preview/2026-04-03/timeline_preview.mp4",
		"/media-audio/2026-04-03/timeline.wav",
		"data-media-player",
		"details-media-player-video",
		`data-player-mode="audio-fallback"`,
		`data-segment-index="0"`,
		`data-start="0.000"`,
		`data-end="1.400"`,
		`data-seek="2.100"`,
		"00:00:00.000",
		"00:00:01.400",
		"Уверенность 0.87",
		"Технические детали",
		"small",
		"Для длинных файлов модель small на CPU может работать очень долго.",
		"test-agent",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("transcript page body missing %q", want)
		}
	}
}

func TestUploadHandler_CreateToggleDeleteTriggerRule(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	createBody := strings.NewReader("name=Test+Rule&category=support&pattern=speak+to+a+manager&match_mode=contains")
	createReq := httptest.NewRequest(http.MethodPost, "/trigger-rules", createBody)
	createReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	createRec := httptest.NewRecorder()

	app.router.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusSeeOther {
		t.Fatalf("create trigger rule status = %d, want %d", createRec.Code, http.StatusSeeOther)
	}

	ruleRepo := repositories.NewTriggerRuleRepository(app.db)
	rules, err := ruleRepo.List(context.Background())
	if err != nil {
		t.Fatalf("List(trigger rules) error = %v", err)
	}

	var created domaintrigger.Rule
	for _, item := range rules {
		if item.Name == "Test Rule" {
			created = item
			break
		}
	}
	if created.ID == 0 {
		t.Fatalf("created rule not found in %#v", rules)
	}

	toggleBody := strings.NewReader("enabled=false")
	toggleReq := httptest.NewRequest(http.MethodPost, "/trigger-rules/"+strconv.FormatInt(created.ID, 10)+"/toggle", toggleBody)
	toggleReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	toggleRec := httptest.NewRecorder()
	app.router.ServeHTTP(toggleRec, toggleReq)

	if toggleRec.Code != http.StatusSeeOther {
		t.Fatalf("toggle trigger rule status = %d, want %d", toggleRec.Code, http.StatusSeeOther)
	}

	rules, err = ruleRepo.List(context.Background())
	if err != nil {
		t.Fatalf("List(trigger rules after toggle) error = %v", err)
	}
	for _, item := range rules {
		if item.ID == created.ID && item.Enabled {
			t.Fatalf("rule %d still enabled after toggle", created.ID)
		}
	}

	deleteReq := httptest.NewRequest(http.MethodPost, "/trigger-rules/"+strconv.FormatInt(created.ID, 10)+"/delete", nil)
	deleteRec := httptest.NewRecorder()
	app.router.ServeHTTP(deleteRec, deleteReq)

	if deleteRec.Code != http.StatusSeeOther {
		t.Fatalf("delete trigger rule status = %d, want %d", deleteRec.Code, http.StatusSeeOther)
	}

	rules, err = ruleRepo.List(context.Background())
	if err != nil {
		t.Fatalf("List(trigger rules after delete) error = %v", err)
	}
	for _, item := range rules {
		if item.ID == created.ID {
			t.Fatalf("rule %d still exists after delete", created.ID)
		}
	}
}

func TestUploadHandler_TranscriptPageShowsTriggerEmptyState(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	ctx := context.Background()
	mediaRepo := repositories.NewMediaRepository(app.db)
	jobRepo := repositories.NewJobRepository(app.db)
	transcriptRepo := repositories.NewTranscriptRepository(app.db)

	nowUTC := time.Date(2026, 4, 3, 19, 0, 0, 0, time.UTC)
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName: "no-triggers.wav",
		StoredName:   "no-triggers.wav",
		Extension:    ".wav",
		MIMEType:     "audio/wav",
		SizeBytes:    1024,
		StoragePath:  "2026-04-03/no-triggers.wav",
		Status:       media.StatusTranscribed,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}
	createUploadedMediaFile(t, app.uploadDir, "2026-04-03/no-triggers.wav", []byte("audio"))
	if _, err := jobRepo.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeAnalyzeTriggers,
		Status:       job.StatusDone,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	}); err != nil {
		t.Fatalf("Create(analyze job) error = %v", err)
	}
	if err := transcriptRepo.Save(ctx, transcript.Transcript{
		MediaID:      mediaID,
		Language:     "en",
		FullText:     "just a normal conversation",
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
		Segments: []transcript.Segment{
			{StartSec: 0, EndSec: 2, Text: "just a normal conversation"},
		},
	}); err != nil {
		t.Fatalf("Save(transcript) error = %v", err)
	}
	if err := mediaRepo.MarkTranscribed(ctx, mediaID, "just a normal conversation", nowUTC); err != nil {
		t.Fatalf("MarkTranscribed() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/media/"+strconv.FormatInt(mediaID, 10)+"/transcript", nil)
	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("transcript page status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "Для этой расшифровки триггеры не найдены.") {
		t.Fatalf("transcript page body missing trigger empty state: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "details-media-player-audio") {
		t.Fatalf("transcript page body missing audio player: %s", rec.Body.String())
	}
}

func TestUploadHandler_TranscriptPageShowsPlayerFallbackWhenSourceMissing(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	ctx := context.Background()
	mediaRepo := repositories.NewMediaRepository(app.db)
	transcriptRepo := repositories.NewTranscriptRepository(app.db)

	nowUTC := time.Date(2026, 4, 3, 19, 15, 0, 0, time.UTC)
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName: "missing-video.mp4",
		StoredName:   "missing-video.mp4",
		Extension:    ".mp4",
		MIMEType:     "video/mp4",
		SizeBytes:    1024,
		StoragePath:  "2026-04-03/missing-video.mp4",
		Status:       media.StatusTranscribed,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}
	if err := transcriptRepo.Save(ctx, transcript.Transcript{
		MediaID:      mediaID,
		Language:     "ru",
		FullText:     "С„Р°Р№Р» Р±РµР· РёСЃС…РѕРґРЅРёРєР°",
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
		Segments: []transcript.Segment{
			{StartSec: 0, EndSec: 1.5, Text: "С„Р°Р№Р» Р±РµР· РёСЃС…РѕРґРЅРёРєР°"},
		},
	}); err != nil {
		t.Fatalf("Save(transcript) error = %v", err)
	}
	if err := mediaRepo.MarkTranscribed(ctx, mediaID, "С„Р°Р№Р» Р±РµР· РёСЃС…РѕРґРЅРёРєР°", nowUTC); err != nil {
		t.Fatalf("MarkTranscribed() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/media/"+strconv.FormatInt(mediaID, 10)+"/transcript", nil)
	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("transcript page status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if strings.Contains(body, "Browser-safe preview") {
		if strings.Contains(body, "data-media-player") {
			t.Fatalf("transcript page unexpectedly rendered media player: %s", body)
		}
		return
	}
	if !strings.Contains(body, "Р’РёРґРµРѕС„Р°Р№Р» СЃРµР№С‡Р°СЃ РЅРµРґРѕСЃС‚СѓРїРµРЅ РґР»СЏ РІСЃС‚СЂРѕРµРЅРЅРѕРіРѕ РїСЂРѕРёРіСЂС‹РІР°С‚РµР»СЏ") {
		t.Fatalf("transcript page body missing player fallback: %s", body)
	}
	if strings.Contains(body, "data-media-player") {
		t.Fatalf("transcript page unexpectedly rendered media player: %s", body)
	}
}

func TestUploadHandler_RequestSummaryCreatesJobAndRedirects(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	ctx := context.Background()
	mediaRepo := repositories.NewMediaRepository(app.db)
	transcriptRepo := repositories.NewTranscriptRepository(app.db)
	jobRepo := repositories.NewJobRepository(app.db)

	nowUTC := time.Date(2026, 4, 3, 19, 30, 0, 0, time.UTC)
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName: "summary-me.wav",
		StoredName:   "summary-me.wav",
		Extension:    ".wav",
		MIMEType:     "audio/wav",
		SizeBytes:    1024,
		StoragePath:  "2026-04-03/summary-me.wav",
		Status:       media.StatusTranscribed,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}
	if err := transcriptRepo.Save(ctx, transcript.Transcript{
		MediaID:      mediaID,
		Language:     "ru",
		FullText:     "РљРѕСЂРѕС‚РєРёР№ С‚РµРєСЃС‚ РґР»СЏ СЃР°РјРјР°СЂРё.",
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
		Segments: []transcript.Segment{
			{StartSec: 0, EndSec: 1, Text: "РљРѕСЂРѕС‚РєРёР№ С‚РµРєСЃС‚ РґР»СЏ СЃР°РјРјР°СЂРё."},
		},
	}); err != nil {
		t.Fatalf("Save(transcript) error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/media/"+strconv.FormatInt(mediaID, 10)+"/summary", nil)
	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("summary request status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if location := rec.Header().Get("Location"); location != "/media/"+strconv.FormatInt(mediaID, 10)+"/transcript?summary_status=requested" {
		t.Fatalf("redirect location = %q, want summary requested redirect", location)
	}

	latestJob, ok, err := jobRepo.FindLatestByMediaAndType(ctx, mediaID, job.TypeGenerateSummary)
	if err != nil {
		t.Fatalf("FindLatestByMediaAndType() error = %v", err)
	}
	if !ok {
		t.Fatal("generate summary job not found")
	}
	if latestJob.Status != job.StatusPending {
		t.Fatalf("summary job status = %q, want %q", latestJob.Status, job.StatusPending)
	}
}

func TestUploadHandler_IndexShowsRussianStepFlow(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("index status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Media Pipeline",
		"Загрузка файла",
		"Активная задача",
		"Последние задачи",
		"Настройки",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("index body missing %q", want)
		}
	}
	if !strings.Contains(body, `id="settings-sheet"`) || !strings.Contains(body, "hidden inert") {
		t.Fatalf("settings sheet should be hidden by default: %s", body)
	}
}

func TestUploadHandler_IndexShowsSettingsBehindGearAndFailedStage(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	ctx := context.Background()
	mediaRepo := repositories.NewMediaRepository(app.db)
	jobRepo := repositories.NewJobRepository(app.db)

	nowUTC := time.Date(2026, 4, 3, 18, 30, 0, 0, time.UTC)
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName: "failed-small.mp4",
		StoredName:   "failed-small.mp4",
		Extension:    ".mp4",
		MIMEType:     "video/mp4",
		SizeBytes:    2048,
		StoragePath:  "2026-04-03/failed-small.mp4",
		Status:       media.StatusFailed,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}
	if _, err := jobRepo.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeExtractAudio,
		Status:       job.StatusDone,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	}); err != nil {
		t.Fatalf("Create(extract job) error = %v", err)
	}
	if _, err := jobRepo.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeTranscribe,
		Status:       job.StatusFailed,
		ErrorMessage: "Не удалось распознать текст: модель small вернула ошибку. Подробности смотрите в логах worker.",
		CreatedAtUTC: nowUTC.Add(time.Minute),
		UpdatedAtUTC: nowUTC.Add(time.Minute),
	}); err != nil {
		t.Fatalf("Create(transcribe job) error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("index status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"settings-sheet",
		"Распознавание и правила",
		"Ошибка на этапе: Распознавание текста",
		"Причина: Не удалось распознать текст: модель small вернула ошибку.",
		"Подробности смотрите в логах worker.",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("index body missing %q", want)
		}
	}
}

func TestUploadHandler_IndexShowsAdaptiveSettingsWarningForHeavyCPUProfile(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	form := strings.NewReader("backend=faster-whisper&model_name=small&device=cpu&compute_type=float32&language=ru&beam_size=5&vad_enabled=on")
	saveReq := httptest.NewRequest(http.MethodPost, "/settings/transcription", form)
	saveReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	saveRec := httptest.NewRecorder()
	app.router.ServeHTTP(saveRec, saveReq)

	if saveRec.Code != http.StatusSeeOther {
		t.Fatalf("save settings status = %d, want %d", saveRec.Code, http.StatusSeeOther)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("index status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Для длинных файлов модель small на CPU может работать очень долго.",
		"Если доступна CUDA, для такой модели лучше использовать её вместо CPU.",
		"Режим float32 на CPU обычно медленнее, чем int8.",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("index body missing %q", want)
		}
	}
}

func TestUploadHandler_TranscriptPageShowsPipelineFailureDetails(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	ctx := context.Background()
	mediaRepo := repositories.NewMediaRepository(app.db)
	jobRepo := repositories.NewJobRepository(app.db)

	nowUTC := time.Date(2026, 4, 3, 18, 45, 0, 0, time.UTC)
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName:       "broken-small.wav",
		StoredName:         "broken-small.wav",
		Extension:          ".wav",
		MIMEType:           "audio/wav",
		SizeBytes:          1024,
		StoragePath:        "2026-04-03/broken-small.wav",
		ExtractedAudioPath: "2026-04-03/broken-small.wav",
		Status:             media.StatusFailed,
		CreatedAtUTC:       nowUTC,
		UpdatedAtUTC:       nowUTC,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}
	if _, err := jobRepo.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeExtractAudio,
		Status:       job.StatusDone,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	}); err != nil {
		t.Fatalf("Create(extract job) error = %v", err)
	}
	if _, err := jobRepo.Create(ctx, job.Job{
		MediaID: mediaID,
		Type:    job.TypeTranscribe,
		Payload: mustEncodeTranscribePayload(t, job.TranscribePayload{
			Settings: domaintranscription.Settings{
				Backend:     domaintranscription.BackendFasterWhisper,
				ModelName:   "small",
				Device:      "cpu",
				ComputeType: "int8",
				Language:    "ru",
				BeamSize:    5,
				VADEnabled:  true,
			},
		}),
		Status:       job.StatusFailed,
		ErrorMessage: "Не удалось распознать текст: faster-whisper завершился с ошибкой small model. Подробности смотрите в логах worker.",
		CreatedAtUTC: nowUTC.Add(time.Minute),
		UpdatedAtUTC: nowUTC.Add(time.Minute),
	}); err != nil {
		t.Fatalf("Create(transcribe job) error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/media/"+strconv.FormatInt(mediaID, 10)+"/transcript", nil)
	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("transcript page status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Состояние",
		"Ошибка на шаге: Распознавание текста",
		"Причина: Не удалось распознать текст: faster-whisper завершился с ошибкой small model.",
		"Подробности смотрите в логах worker.",
		"Технические детали",
		"Политика времени",
		"Для этого файла лимит распознавания автоматически увеличен до 9 ч 15 мин.",
		"Длительность аудио",
		"1 ч",
		"Класс файла",
		"Длинный файл",
		"Расчётный лимит",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("transcript page body missing %q", want)
		}
	}
}

func TestUploadHandler_DeleteMediaRemovesRowsAndFiles(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	ctx := context.Background()
	mediaRepo := repositories.NewMediaRepository(app.db)
	jobRepo := repositories.NewJobRepository(app.db)
	transcriptRepo := repositories.NewTranscriptRepository(app.db)
	triggerEventRepo := repositories.NewTriggerEventRepository(app.db)
	triggerRuleRepo := repositories.NewTriggerRuleRepository(app.db)
	triggerScreenshotRepo := repositories.NewTriggerScreenshotRepository(app.db)
	summaryRepo := repositories.NewSummaryRepository(app.db)

	nowUTC := time.Date(2026, 4, 3, 17, 0, 0, 0, time.UTC)
	uploadRelative := filepath.ToSlash(filepath.Join("2026-04-03", "delete-me.wav"))
	audioRelative := filepath.ToSlash(filepath.Join("2026-04-03", "delete-me-audio.wav"))
	screenshotRelative := filepath.ToSlash(filepath.Join("2026-04-03", "delete-me-screenshot.jpg"))
	if err := os.MkdirAll(filepath.Join(app.uploadDir, "2026-04-03"), 0o755); err != nil {
		t.Fatalf("MkdirAll(uploadDir) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(app.audioDir, "2026-04-03"), 0o755); err != nil {
		t.Fatalf("MkdirAll(audioDir) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(app.screenshotsDir, "2026-04-03"), 0o755); err != nil {
		t.Fatalf("MkdirAll(screenshotsDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(app.uploadDir, filepath.FromSlash(uploadRelative)), []byte("upload"), 0o644); err != nil {
		t.Fatalf("WriteFile(upload) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(app.audioDir, filepath.FromSlash(audioRelative)), []byte("audio"), 0o644); err != nil {
		t.Fatalf("WriteFile(audio) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(app.screenshotsDir, filepath.FromSlash(screenshotRelative)), pngPixelBytes(), 0o644); err != nil {
		t.Fatalf("WriteFile(screenshot) error = %v", err)
	}

	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName:       "delete-me.wav",
		StoredName:         "delete-me.wav",
		Extension:          ".wav",
		MIMEType:           "audio/wav",
		SizeBytes:          1024,
		StoragePath:        uploadRelative,
		ExtractedAudioPath: audioRelative,
		Status:             media.StatusTranscribed,
		CreatedAtUTC:       nowUTC,
		UpdatedAtUTC:       nowUTC,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}
	if _, err := jobRepo.Create(ctx, job.Job{
		MediaID:      mediaID,
		Type:         job.TypeTranscribe,
		Status:       job.StatusDone,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	}); err != nil {
		t.Fatalf("Create(job) error = %v", err)
	}
	if err := transcriptRepo.Save(ctx, transcript.Transcript{
		MediaID:      mediaID,
		Language:     "en",
		FullText:     "delete me",
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
		Segments: []transcript.Segment{
			{StartSec: 0, EndSec: 1, Text: "delete me"},
		},
	}); err != nil {
		t.Fatalf("Save(transcript) error = %v", err)
	}
	transcriptItem, ok, err := transcriptRepo.GetByMediaID(ctx, mediaID)
	if err != nil {
		t.Fatalf("GetByMediaID(transcript) error = %v", err)
	}
	if !ok {
		t.Fatal("GetByMediaID(transcript) ok = false, want true")
	}
	triggerRules, err := triggerRuleRepo.List(ctx)
	if err != nil {
		t.Fatalf("List(trigger rules) error = %v", err)
	}
	if len(triggerRules) == 0 {
		t.Fatal("trigger rules are empty, want seeded rules")
	}
	if err := triggerEventRepo.ReplaceForMedia(ctx, mediaID, &transcriptItem.ID, []domaintrigger.Event{
		{
			MediaID:      mediaID,
			TranscriptID: &transcriptItem.ID,
			RuleID:       triggerRules[0].ID,
			Category:     "billing",
			MatchedText:  "refund",
			SegmentIndex: 0,
			StartSec:     0,
			EndSec:       1,
			SegmentText:  "delete me",
			ContextText:  "delete me",
			CreatedAtUTC: nowUTC,
		},
	}); err != nil {
		t.Fatalf("ReplaceForMedia(trigger events) error = %v", err)
	}
	triggerEvents, err := triggerEventRepo.ListByMediaID(ctx, mediaID)
	if err != nil {
		t.Fatalf("ListByMediaID(trigger events) error = %v", err)
	}
	if err := triggerScreenshotRepo.ReplaceForMedia(ctx, mediaID, []domaintrigger.Screenshot{
		{
			MediaID:        mediaID,
			TriggerEventID: triggerEvents[0].ID,
			TimestampSec:   0,
			ImagePath:      screenshotRelative,
			Width:          1,
			Height:         1,
			CreatedAtUTC:   nowUTC,
		},
	}); err != nil {
		t.Fatalf("ReplaceForMedia(trigger screenshots) error = %v", err)
	}
	if err := summaryRepo.Save(ctx, domainsummary.Summary{
		MediaID:      mediaID,
		SummaryText:  "РљРѕСЂРѕС‚РєРѕРµ СЃР°РјРјР°СЂРё РґР»СЏ СѓРґР°Р»РµРЅРёСЏ.",
		Highlights:   []string{"РўРµР·РёСЃ 1"},
		Provider:     "simple-summary-v1",
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	}); err != nil {
		t.Fatalf("Save(summary) error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/media/"+strconv.FormatInt(mediaID, 10)+"/delete", nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	rec := httptest.NewRecorder()

	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Status != "deleted" {
		t.Fatalf("status = %q, want deleted", payload.Status)
	}

	assertDBCount(t, app.db, "SELECT COUNT(*) FROM media WHERE id = ?", mediaID, 0)
	assertDBCount(t, app.db, "SELECT COUNT(*) FROM jobs WHERE media_id = ?", mediaID, 0)
	assertDBCount(t, app.db, "SELECT COUNT(*) FROM transcripts WHERE media_id = ?", mediaID, 0)
	assertDBCount(t, app.db, "SELECT COUNT(*) FROM summaries WHERE media_id = ?", mediaID, 0)

	if _, err := os.Stat(filepath.Join(app.uploadDir, filepath.FromSlash(uploadRelative))); !os.IsNotExist(err) {
		t.Fatalf("uploaded file still exists, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(app.audioDir, filepath.FromSlash(audioRelative))); !os.IsNotExist(err) {
		t.Fatalf("audio file still exists, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(app.screenshotsDir, filepath.FromSlash(screenshotRelative))); !os.IsNotExist(err) {
		t.Fatalf("screenshot file still exists, stat err = %v", err)
	}
}

func TestUploadHandler_IndexShowsTranscriptLinkForTranscribingItem(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	ctx := context.Background()
	mediaRepo := repositories.NewMediaRepository(app.db)

	nowUTC := time.Date(2026, 4, 3, 18, 0, 0, 0, time.UTC)
	mediaID, err := mediaRepo.Create(ctx, media.Media{
		OriginalName: "in-progress.wav",
		StoredName:   "in-progress.wav",
		Extension:    ".wav",
		MIMEType:     "audio/wav",
		SizeBytes:    2048,
		StoragePath:  "2026-04-03/in-progress.wav",
		Status:       media.StatusTranscribing,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	})
	if err != nil {
		t.Fatalf("Create(media) error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("index status = %d, want %d", rec.Code, http.StatusOK)
	}
	wantLink := "/media/" + strconv.FormatInt(mediaID, 10) + "/transcript"
	if !strings.Contains(rec.Body.String(), wantLink) {
		t.Fatalf("index page does not contain transcript link %q", wantLink)
	}
}

func newTestRouter(t *testing.T) http.Handler {
	return newTestApp(t).router
}

type testWebApp struct {
	router         http.Handler
	db             *sql.DB
	uploadDir      string
	audioDir       string
	previewDir     string
	screenshotsDir string
}

type stubHandlerAudioDurationReader struct {
	duration time.Duration
	err      error
}

func (s stubHandlerAudioDurationReader) ReadDuration(string) (time.Duration, error) {
	if s.err != nil {
		return 0, s.err
	}
	return s.duration, nil
}

func newTestApp(t *testing.T) testWebApp {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "app.db")
	uploadDir := filepath.Join(tempDir, "uploads")
	audioDir := filepath.Join(tempDir, "audio")
	previewDir := filepath.Join(tempDir, "previews")
	screenshotsDir := filepath.Join(tempDir, "screenshots")

	sqlDB, err := db.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	migrationsPath, err := infraRuntime.ResolvePath("internal/infra/db/migrations")
	if err != nil {
		t.Fatalf("ResolvePath(migrations) error = %v", err)
	}
	if err := db.RunMigrations(sqlDB, migrationsPath); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	templatesDir, err := infraRuntime.ResolvePath("internal/transport/http/views/templates")
	if err != nil {
		t.Fatalf("ResolvePath(template dir) error = %v", err)
	}
	staticPath, err := infraRuntime.ResolvePath("web/static")
	if err != nil {
		t.Fatalf("ResolvePath(static) error = %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mediaRepo := repositories.NewMediaRepository(sqlDB)
	jobRepo := repositories.NewJobRepository(sqlDB)
	transcriptRepo := repositories.NewTranscriptRepository(sqlDB)
	triggerRuleRepo := repositories.NewTriggerRuleRepository(sqlDB)
	triggerEventRepo := repositories.NewTriggerEventRepository(sqlDB)
	triggerScreenshotRepo := repositories.NewTriggerScreenshotRepository(sqlDB)
	summaryRepo := repositories.NewSummaryRepository(sqlDB)
	uploadStorage := storage.NewLocalStorage(uploadDir)
	audioStorage := storage.NewLocalStorage(audioDir)
	previewStorage := storage.NewLocalStorage(previewDir)
	screenshotStorage := storage.NewLocalStorage(screenshotsDir)
	profileService := transcriptionapp.NewService(
		repositories.NewTranscriptionProfileRepository(sqlDB),
		domaintranscription.DefaultProfile("ru"),
	)
	triggerRuleService := triggerapp.NewService(triggerRuleRepo)
	uploadUC := command.NewUploadMediaUseCase(
		mediaRepo,
		jobRepo,
		uploadStorage,
		10*1024*1024,
		logger,
	)
	audioDurationReader := stubHandlerAudioDurationReader{duration: 60 * time.Minute}
	transcriptViewUC := mediaapp.NewTranscriptViewUseCase(
		mediaRepo,
		transcriptRepo,
		triggerEventRepo,
		triggerScreenshotRepo,
		summaryRepo,
		jobRepo,
		uploadDir,
		audioDurationReader,
		audioDir,
		previewDir,
		5*time.Minute,
	)
	requestSummaryUC := mediaapp.NewRequestSummaryUseCase(mediaRepo, transcriptRepo, jobRepo)
	deleteMediaUC := mediaapp.NewDeleteMediaUseCase(mediaRepo, triggerScreenshotRepo, uploadStorage, audioStorage, previewStorage, screenshotStorage, logger)
	handler, err := handlers.NewUploadHandler(
		uploadUC,
		profileService,
		triggerRuleService,
		transcriptViewUC,
		requestSummaryUC,
		deleteMediaUC,
		jobRepo,
		templatesDir,
		10*1024*1024,
		logger,
	)
	if err != nil {
		t.Fatalf("NewUploadHandler() error = %v", err)
	}

	machineAPIHandler := handlers.NewMachineAPIHandler(transcriptViewUC, logger)
	triggerRuleHandler := handlers.NewTriggerRuleHandler(triggerRuleService, handler, logger)

	return testWebApp{
		router:         httptransport.NewRouter(logger, handler, machineAPIHandler, triggerRuleHandler, staticPath, uploadDir, audioDir, previewDir, screenshotsDir, "", 30*time.Second),
		db:             sqlDB,
		uploadDir:      uploadDir,
		audioDir:       audioDir,
		previewDir:     previewDir,
		screenshotsDir: screenshotsDir,
	}
}

func assertDBCount(t *testing.T, sqlDB *sql.DB, query string, mediaID int64, want int) {
	t.Helper()

	var count int
	if err := sqlDB.QueryRowContext(context.Background(), query, mediaID).Scan(&count); err != nil {
		t.Fatalf("QueryRow(%q) error = %v", query, err)
	}
	if count != want {
		t.Fatalf("count for %q = %d, want %d", query, count, want)
	}
}

func testWAVBytes() []byte {
	return []byte{
		'R', 'I', 'F', 'F',
		0x24, 0x08, 0x00, 0x00,
		'W', 'A', 'V', 'E',
		'f', 'm', 't', ' ',
		0x10, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x01, 0x00,
		0x44, 0xAC, 0x00, 0x00,
		0x88, 0x58, 0x01, 0x00,
		0x02, 0x00, 0x10, 0x00,
		'd', 'a', 't', 'a',
		0x00, 0x08, 0x00, 0x00,
	}
}

func pngPixelBytes() []byte {
	return []byte{
		0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n',
		0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
		0x00, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00,
		0x90, 0x77, 0x53, 0xde,
		0x00, 0x00, 0x00, 0x0c, 'I', 'D', 'A', 'T',
		0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00, 0x00, 0x03, 0x01, 0x01, 0x00,
		0x18, 0xdd, 0x8d, 0xb3,
		0x00, 0x00, 0x00, 0x00, 'I', 'E', 'N', 'D', 0xae, 0x42, 0x60, 0x82,
	}
}

func createUploadedMediaFile(t *testing.T, uploadDir string, relativePath string, content []byte) {
	t.Helper()

	fullPath := filepath.Join(uploadDir, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(uploaded media dir) error = %v", err)
	}
	if err := os.WriteFile(fullPath, content, 0o644); err != nil {
		t.Fatalf("WriteFile(uploaded media) error = %v", err)
	}
}

func mustEncodeTranscribePayload(t *testing.T, payload job.TranscribePayload) string {
	t.Helper()

	raw, err := job.EncodeTranscribePayload(payload)
	if err != nil {
		t.Fatalf("EncodeTranscribePayload() error = %v", err)
	}

	return raw
}
