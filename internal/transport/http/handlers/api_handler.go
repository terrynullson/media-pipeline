package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/transcription"
	domaintrigger "media-pipeline/internal/domain/trigger"
	"media-pipeline/internal/observability"
)

type apiMetric struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Tone  string `json:"tone"`
	Help  string `json:"help"`
}

type apiNotice struct {
	Label string `json:"label"`
	Tone  string `json:"tone"`
	Text  string `json:"text"`
}

type apiMediaItem struct {
	ID                 int64              `json:"id"`
	Name               string             `json:"name"`
	Extension          string             `json:"extension"`
	SizeHuman          string             `json:"sizeHuman"`
	CreatedAtUTC       string             `json:"createdAtUtc"`
	Status             string             `json:"status"`
	StatusLabel        string             `json:"statusLabel"`
	StatusTone         string             `json:"statusTone"`
	StageLabel         string             `json:"stageLabel"`
	StageValue         int                `json:"stageValue"`
	StageTotal         int                `json:"stageTotal"`
	StagePercent       int                `json:"stagePercent"`
	CurrentStage       string             `json:"currentStage"`
	CurrentTimingText  string             `json:"currentTimingText"`
	ErrorSummary       string             `json:"errorSummary,omitempty"`
	HasTranscript      bool               `json:"hasTranscript"`
	IsAudioOnly        bool               `json:"isAudioOnly"`
	PreviewReady       bool               `json:"previewReady"`
	TranscriptURL      string             `json:"transcriptUrl"`
	DeleteURL          string             `json:"deleteUrl"`
	PipelineSteps      []PipelineStepView `json:"pipelineSteps"`
	PreviewStatusLabel string             `json:"previewStatusLabel,omitempty"`
	PreviewStatusTone  string             `json:"previewStatusTone,omitempty"`
}

type apiJobItem struct {
	ID                int64  `json:"id"`
	MediaID           int64  `json:"mediaId"`
	MediaName         string `json:"mediaName"`
	Type              string `json:"type"`
	TypeLabel         string `json:"typeLabel"`
	Status            string `json:"status"`
	StatusLabel       string `json:"statusLabel"`
	StatusTone        string `json:"statusTone"`
	CreatedAtUTC      string `json:"createdAtUtc"`
	StartedAtUTC      string `json:"startedAtUtc,omitempty"`
	FinishedAtUTC     string `json:"finishedAtUtc,omitempty"`
	DurationLabel     string `json:"durationLabel,omitempty"`
	Attempts          int    `json:"attempts"`
	ProgressPercent   *int   `json:"progressPercent,omitempty"`
	ProgressLabel     string `json:"progressLabel,omitempty"`
	ErrorMessage      string `json:"errorMessage,omitempty"`
	DetailURL         string `json:"detailUrl"`
	MediaStatusLabel  string `json:"mediaStatusLabel"`
	MediaStageLabel   string `json:"mediaStageLabel"`
	MediaCurrentStage string `json:"mediaCurrentStage"`
}

type transcriptionSettingsPayload struct {
	Backend     string `json:"backend"`
	ModelName   string `json:"modelName"`
	Device      string `json:"device"`
	ComputeType string `json:"computeType"`
	Language    string `json:"language"`
	BeamSize    int    `json:"beamSize"`
	VADEnabled  bool   `json:"vadEnabled"`
	UITheme     string `json:"uiTheme"`
}

type uiPreferencePayload struct {
	UITheme string `json:"uiTheme"`
}

type triggerRulePayload struct {
	Name      string `json:"name"`
	Category  string `json:"category"`
	Pattern   string `json:"pattern"`
	MatchMode string `json:"matchMode"`
	Enabled   *bool  `json:"enabled,omitempty"`
}

func (h *UploadHandler) APIDashboard(w http.ResponseWriter, r *http.Request) {
	viewItems, err := h.buildMediaListItems(r.Context())
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("load dashboard media failed", slog.Any("error", err))
		http.Error(w, "не удалось загрузить dashboard", http.StatusInternalServerError)
		return
	}

	mediaItems := make([]apiMediaItem, 0, len(viewItems))
	for _, item := range viewItems {
		mediaItems = append(mediaItems, apiMediaFromView(item))
	}

	jobItems, err := h.buildAPIJobItems(r.Context(), viewItems)
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("load dashboard jobs failed", slog.Any("error", err))
		http.Error(w, "не удалось загрузить jobs", http.StatusInternalServerError)
		return
	}

	inProgress := 0
	failedMedia := 0
	previewReady := 0
	transcriptReady := 0
	for _, item := range mediaItems {
		if item.Status == "failed" {
			failedMedia++
		}
		if strings.Contains(strings.ToLower(item.StatusLabel), "работ") {
			inProgress++
		}
		if item.PreviewReady {
			previewReady++
		}
		if item.HasTranscript {
			transcriptReady++
		}
	}

	pendingJobs := 0
	runningJobs := 0
	doneJobs := 0
	failedJobs := make([]apiJobItem, 0)
	for _, item := range jobItems {
		switch item.Status {
		case string(job.StatusPending):
			pendingJobs++
		case string(job.StatusRunning):
			runningJobs++
		case string(job.StatusDone):
			doneJobs++
		case string(job.StatusFailed):
			failedJobs = append(failedJobs, item)
		}
	}
	if len(failedJobs) > 5 {
		failedJobs = failedJobs[:5]
	}

	workerNotice := apiNotice{
		Label: "Worker",
		Tone:  "success",
		Text:  "Новый frontend работает поверх JSON API, а старые HTML-шаблоны пока не трогаем.",
	}
	if runningJobs > 0 {
		workerNotice.Tone = "running"
		workerNotice.Text = fmt.Sprintf("Сейчас worker выполняет %d job. Очередь продолжает жить отдельно от UI.", runningJobs)
	}
	if len(failedJobs) > 0 {
		workerNotice.Tone = "error"
		workerNotice.Text = "Есть job с ошибками. Полезные сообщения проброшены в API и показаны в dashboard."
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"overview": []apiMetric{
			{Label: "Media", Value: strconv.Itoa(len(mediaItems)), Tone: "neutral", Help: "Файлы в витрине"},
			{Label: "Jobs in progress", Value: strconv.Itoa(inProgress), Tone: "running", Help: "Обработка ещё идёт"},
			{Label: "Failed media", Value: strconv.Itoa(failedMedia), Tone: "error", Help: "Нужна проверка лога"},
			{Label: "Transcript ready", Value: strconv.Itoa(transcriptReady), Tone: "success", Help: "Текст уже доступен"},
		},
		"queueBreakdown": []apiMetric{
			{Label: "Pending", Value: strconv.Itoa(pendingJobs), Tone: "neutral", Help: "Ждут worker"},
			{Label: "Running", Value: strconv.Itoa(runningJobs), Tone: "running", Help: "Выполняются"},
			{Label: "Done", Value: strconv.Itoa(doneJobs), Tone: "success", Help: "Завершены"},
			{Label: "Preview ready", Value: strconv.Itoa(previewReady), Tone: "success", Help: "Есть browser-safe preview"},
		},
		"workerNotice": workerNotice,
		"recentMedia":  mediaItems[:minInt(len(mediaItems), 6)],
		"recentJobs":   jobItems[:minInt(len(jobItems), 8)],
		"latestErrors": failedJobs,
	})
}

func (h *UploadHandler) APIMediaList(w http.ResponseWriter, r *http.Request) {
	viewItems, err := h.buildMediaListItems(r.Context())
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("load api media list failed", slog.Any("error", err))
		http.Error(w, "не удалось загрузить media list", http.StatusInternalServerError)
		return
	}

	items := make([]apiMediaItem, 0, len(viewItems))
	for _, item := range viewItems {
		items = append(items, apiMediaFromView(item))
	}

	h.writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *UploadHandler) APIJobsList(w http.ResponseWriter, r *http.Request) {
	viewItems, err := h.buildMediaListItems(r.Context())
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("load api jobs list media failed", slog.Any("error", err))
		http.Error(w, "не удалось загрузить jobs", http.StatusInternalServerError)
		return
	}

	items, err := h.buildAPIJobItems(r.Context(), viewItems)
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("build api jobs list failed", slog.Any("error", err))
		http.Error(w, "не удалось загрузить jobs", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *UploadHandler) APIMediaDetail(w http.ResponseWriter, r *http.Request) {
	mediaID, err := mediaIDFromRequest(r)
	if err != nil {
		http.Error(w, "некорректный media id", http.StatusBadRequest)
		return
	}

	result, err := h.transcriptViewUC.Load(r.Context(), mediaID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		observability.LoggerFromContext(r.Context(), h.logger).Error("load api media detail failed", slog.Int64("media_id", mediaID), slog.Any("error", err))
		http.Error(w, "не удалось загрузить media detail", http.StatusInternalServerError)
		return
	}

	jobs := make([]job.Job, 0, 6)
	for _, current := range []*job.Job{result.ExtractAudioJob, result.PreviewJob, result.TranscribeJob, result.AnalyzeJob, result.ScreenshotJob, result.SummaryJob} {
		if current != nil {
			jobs = append(jobs, *current)
		}
	}
	pipelineView := buildMediaPipelineView(result.Media, jobs)
	playerView := buildTranscriptPlayerView(result)
	triggerViews := buildTriggerEventViews(result.TriggerEvents, result.TriggerScreenshots, result.Media, result.ScreenshotJob)
	triggerStatusLabel, triggerStatusTone, triggerNotice, triggerNoticeTone := describeTriggerAnalysis(result.AnalyzeJob, len(triggerViews))
	summaryStatusLabel, summaryStatusTone, summaryNotice, summaryNoticeTone := describeSummaryState(result.SummaryJob, result.HasSummary)
	showSummaryAction, summaryActionLabel := summaryActionState(result.SummaryJob, result.HasTranscript, result.HasSummary)

	h.writeJSON(w, http.StatusOK, map[string]any{
		"media": map[string]any{
			"id":           result.Media.ID,
			"name":         result.Media.OriginalName,
			"extension":    result.Media.Extension,
			"mimeType":     result.Media.MIMEType,
			"sizeHuman":    HumanSize(result.Media.SizeBytes),
			"createdAtUtc": FormatDateTimeUTC(result.Media.CreatedAtUTC),
			"isAudioOnly":  result.Media.IsAudioOnly(),
		},
		"pipeline": pipelineView,
		"player":   playerView,
		"transcript": map[string]any{
			"hasTranscript":      result.HasTranscript,
			"fullTextParagraphs": buildTranscriptParagraphs(result.Transcript.Segments, result.Transcript.FullText),
			"segments":           buildTranscriptSegments(result.Transcript.Segments),
		},
		"triggers": map[string]any{
			"statusLabel": triggerStatusLabel,
			"statusTone":  triggerStatusTone,
			"notice":      triggerNotice,
			"noticeTone":  triggerNoticeTone,
			"items":       triggerViews,
		},
		"summary": map[string]any{
			"hasSummary":        result.HasSummary,
			"text":              strings.TrimSpace(result.Summary.SummaryText),
			"highlights":        append([]string(nil), result.Summary.Highlights...),
			"provider":          fallbackValue(result.Summary.Provider, "не указан"),
			"updatedAtUtc":      FormatDateTimeUTC(result.Summary.UpdatedAtUTC),
			"statusLabel":       summaryStatusLabel,
			"statusTone":        summaryStatusTone,
			"notice":            summaryNotice,
			"noticeTone":        summaryNoticeTone,
			"showAction":        showSummaryAction,
			"actionLabel":       summaryActionLabel,
			"requestSummaryUrl": fmt.Sprintf("/media/%d/summary", result.Media.ID),
		},
		"settingsSnapshot": map[string]any{
			"settings":            buildTranscriptSettings(result.Settings),
			"settingsWarnings":    buildTranscriptSettingsWarnings(result.Settings),
			"settingsUnavailable": result.SettingsUnavailable,
			"runtimePolicy":       buildTranscriptRuntimePolicyView(result),
			"runtimeSnapshot":     buildRuntimeSnapshotItems(result.Media.RuntimeSnapshotJSON),
		},
		"actions": map[string]string{
			"deleteUrl":        fmt.Sprintf("/media/%d/delete", result.Media.ID),
			"legacyTranscript": fmt.Sprintf("/media/%d/transcript", result.Media.ID),
		},
	})
}

func (h *UploadHandler) APITranscriptionSettings(w http.ResponseWriter, r *http.Request) {
	profile, err := h.transcriptionSvc.GetCurrent(r.Context())
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("load api transcription settings failed", slog.Any("error", err))
		http.Error(w, "не удалось загрузить настройки", http.StatusInternalServerError)
		return
	}

	form := buildSettingsForm(profile)
	h.writeJSON(w, http.StatusOK, map[string]any{
		"profile":  form,
		"warnings": buildSettingsWarnings(form),
		"ui": map[string]any{
			"theme":           form.UITheme,
			"legacyAppURL":    "/app",
			"modernAppURL":    "/app-v1",
			"preferredAppURL": preferredAppURL(form.UITheme),
			"workspaceURL":    "/workspace",
		},
		"options": map[string]any{
			"backends": backendOptions(),
			"models":   transcription.SupportedModels(),
			"devices":  transcription.SupportedDevices(),
			"cpu":      transcription.SupportedComputeTypes("cpu"),
			"cuda":     transcription.SupportedComputeTypes("cuda"),
			"themes":   []string{"old", "new"},
		},
	})
}

func (h *UploadHandler) APIUpdateTranscriptionSettings(w http.ResponseWriter, r *http.Request) {
	var payload transcriptionSettingsPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "не удалось прочитать JSON настроек", http.StatusBadRequest)
		return
	}

	profile := transcription.NormalizeProfile(transcription.Profile{
		Backend:     transcription.Backend(payload.Backend),
		ModelName:   payload.ModelName,
		Device:      payload.Device,
		ComputeType: payload.ComputeType,
		Language:    payload.Language,
		BeamSize:    payload.BeamSize,
		VADEnabled:  payload.VADEnabled,
		UITheme:     payload.UITheme,
		IsDefault:   true,
	})
	if err := transcription.ValidateProfile(profile); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": err.Error()})
		return
	}

	saved, err := h.transcriptionSvc.SaveCurrent(r.Context(), profile)
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("save api transcription settings failed", slog.Any("error", err))
		http.Error(w, "не удалось сохранить настройки", http.StatusInternalServerError)
		return
	}

	form := buildSettingsForm(saved)
	h.writeJSON(w, http.StatusOK, map[string]any{
		"status":   "saved",
		"profile":  form,
		"warnings": buildSettingsWarnings(form),
		"ui": map[string]any{
			"theme":           form.UITheme,
			"legacyAppURL":    "/app",
			"modernAppURL":    "/app-v1",
			"preferredAppURL": preferredAppURL(form.UITheme),
			"workspaceURL":    "/workspace",
		},
	})
}

func (h *UploadHandler) APIUIConfig(w http.ResponseWriter, r *http.Request) {
	profile, err := h.transcriptionSvc.GetCurrent(r.Context())
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("load api ui config failed", slog.Any("error", err))
		http.Error(w, "не удалось загрузить UI config", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"maxUploadBytes":  h.maxUploadSizeB,
		"maxUploadHuman":  HumanSize(h.maxUploadSizeB),
		"acceptedFormats": []string{".mp4", ".mov", ".mkv", ".avi", ".webm", ".mp3", ".wav", ".m4a", ".aac", ".flac"},
		"uiTheme":         normalizeUITheme(profile.UITheme),
		"legacyAppURL":    "/app",
		"modernAppURL":    "/app-v1",
		"preferredAppURL": preferredAppURL(profile.UITheme),
		"workspaceURL":    "/workspace",
	})
}

func (h *UploadHandler) APIUpdateUITheme(w http.ResponseWriter, r *http.Request) {
	var payload uiPreferencePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "не удалось прочитать JSON UI preference", http.StatusBadRequest)
		return
	}

	profile, err := h.transcriptionSvc.GetCurrent(r.Context())
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("load profile for ui preference failed", slog.Any("error", err))
		http.Error(w, "не удалось загрузить настройки", http.StatusInternalServerError)
		return
	}

	profile.UITheme = normalizeUITheme(payload.UITheme)
	saved, err := h.transcriptionSvc.SaveCurrent(r.Context(), profile)
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("save ui preference failed", slog.Any("error", err))
		http.Error(w, "не удалось сохранить UI preference", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"status":          "saved",
		"uiTheme":         normalizeUITheme(saved.UITheme),
		"preferredAppURL": preferredAppURL(saved.UITheme),
	})
}

func (h *UploadHandler) APITriggerRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.triggerRulesSvc.List(r.Context())
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("load api trigger rules failed", slog.Any("error", err))
		http.Error(w, "не удалось загрузить правила", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{"items": buildTriggerRuleViews(rules)})
}

func (h *UploadHandler) APICreateTriggerRule(w http.ResponseWriter, r *http.Request) {
	var payload triggerRulePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "не удалось прочитать JSON правила", http.StatusBadRequest)
		return
	}

	rule := domaintrigger.NormalizeRule(domaintrigger.Rule{
		Name:      payload.Name,
		Category:  payload.Category,
		Pattern:   payload.Pattern,
		MatchMode: domaintrigger.MatchMode(payload.MatchMode),
		Enabled:   true,
	})
	if err := domaintrigger.ValidateRule(rule); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": err.Error()})
		return
	}

	created, err := h.triggerRulesSvc.Create(r.Context(), rule)
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("create api trigger rule failed", slog.Any("error", err))
		http.Error(w, "не удалось создать правило", http.StatusInternalServerError)
		return
	}

	view := buildTriggerRuleViews([]domaintrigger.Rule{created})
	h.writeJSON(w, http.StatusCreated, map[string]any{"status": "created", "item": view[0]})
}

func (h *UploadHandler) APIUpdateTriggerRule(w http.ResponseWriter, r *http.Request) {
	ruleID, err := triggerRuleIDFromRequest(r)
	if err != nil {
		http.Error(w, "invalid trigger rule id", http.StatusBadRequest)
		return
	}

	var payload triggerRulePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "не удалось прочитать JSON правила", http.StatusBadRequest)
		return
	}
	if payload.Enabled == nil {
		http.Error(w, "нужно передать enabled", http.StatusBadRequest)
		return
	}

	if err := h.triggerRulesSvc.SetEnabled(r.Context(), ruleID, *payload.Enabled); err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("update api trigger rule failed", slog.Int64("rule_id", ruleID), slog.Any("error", err))
		http.Error(w, "не удалось обновить правило", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{"status": "updated", "ruleId": ruleID, "enabled": *payload.Enabled})
}

func (h *UploadHandler) APIDeleteTriggerRule(w http.ResponseWriter, r *http.Request) {
	ruleID, err := triggerRuleIDFromRequest(r)
	if err != nil {
		http.Error(w, "invalid trigger rule id", http.StatusBadRequest)
		return
	}

	if err := h.triggerRulesSvc.Delete(r.Context(), ruleID); err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("delete api trigger rule failed", slog.Int64("rule_id", ruleID), slog.Any("error", err))
		http.Error(w, "не удалось удалить правило", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{"status": "deleted", "ruleId": ruleID})
}

func (h *UploadHandler) buildAPIJobItems(ctx context.Context, viewItems []MediaListItem) ([]apiJobItem, error) {
	items := make([]apiJobItem, 0)
	for _, mediaItem := range viewItems {
		jobs, err := h.jobReader.ListByMediaID(ctx, mediaItem.ID)
		if err != nil {
			return nil, fmt.Errorf("load jobs for media %d: %w", mediaItem.ID, err)
		}

		for _, current := range jobs {
			statusLabel, statusTone := apiJobStatus(current.Status)
			item := apiJobItem{
				ID:                current.ID,
				MediaID:           current.MediaID,
				MediaName:         mediaItem.OriginalName,
				Type:              string(current.Type),
				TypeLabel:         apiJobTypeLabel(current.Type),
				Status:            string(current.Status),
				StatusLabel:       statusLabel,
				StatusTone:        statusTone,
				CreatedAtUTC:      FormatDateTimeUTC(current.CreatedAtUTC),
				Attempts:          current.Attempts,
				ProgressLabel:     strings.TrimSpace(current.ProgressLabel),
				ErrorMessage:      strings.TrimSpace(current.ErrorMessage),
				DetailURL:         mediaItem.TranscriptURL,
				MediaStatusLabel:  mediaItem.StatusLabel,
				MediaStageLabel:   mediaItem.StageLabel,
				MediaCurrentStage: mediaItem.CurrentStage,
			}
			if current.StartedAtUTC != nil {
				item.StartedAtUTC = FormatDateTimeUTC(current.StartedAtUTC.UTC())
			}
			if current.FinishedAtUTC != nil {
				item.FinishedAtUTC = FormatDateTimeUTC(current.FinishedAtUTC.UTC())
			}
			if current.DurationMS != nil {
				item.DurationLabel = FormatDurationRU(time.Duration(*current.DurationMS) * time.Millisecond)
			}
			if current.ProgressPercent != nil {
				value := clampPercent(*current.ProgressPercent)
				item.ProgressPercent = &value
			}
			items = append(items, item)
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAtUTC > items[j].CreatedAtUTC
	})

	return items, nil
}

func apiMediaFromView(item MediaListItem) apiMediaItem {
	extension := strings.ToLower(strings.TrimSpace(item.Extension))
	return apiMediaItem{
		ID:                item.ID,
		Name:              item.OriginalName,
		Extension:         item.Extension,
		SizeHuman:         item.SizeHuman,
		CreatedAtUTC:      item.CreatedAtUTC,
		Status:            string(item.Status),
		StatusLabel:       item.StatusLabel,
		StatusTone:        item.StatusTone,
		StageLabel:        item.StageLabel,
		StageValue:        item.StageValue,
		StageTotal:        item.StageTotal,
		StagePercent:      item.StagePercent,
		CurrentStage:      item.CurrentStage,
		CurrentTimingText: item.CurrentTimingText,
		ErrorSummary:      item.ErrorSummary,
		HasTranscript:     item.HasTranscript,
		IsAudioOnly:       extension == ".wav" || extension == ".mp3" || extension == ".m4a" || extension == ".aac" || extension == ".flac",
		PreviewReady:      false,
		TranscriptURL:     item.TranscriptURL,
		DeleteURL:         item.DeleteURL,
		PipelineSteps:     item.Steps,
	}
}

func apiJobStatus(status job.Status) (string, string) {
	switch status {
	case job.StatusPending:
		return "В очереди", "neutral"
	case job.StatusRunning:
		return "В работе", "running"
	case job.StatusDone:
		return "Готово", "success"
	case job.StatusFailed:
		return "Ошибка", "error"
	default:
		return string(status), "neutral"
	}
}

func apiJobTypeLabel(kind job.Type) string {
	switch kind {
	case job.TypeUpload:
		return "Upload"
	case job.TypeExtractAudio:
		return "Extract audio"
	case job.TypePreparePreviewVideo:
		return "Prepare preview"
	case job.TypeTranscribe:
		return "Transcribe"
	case job.TypeAnalyzeTriggers:
		return "Analyze triggers"
	case job.TypeExtractScreenshots:
		return "Extract screenshots"
	case job.TypeGenerateSummary:
		return "Generate summary"
	default:
		return string(kind)
	}
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func normalizeUITheme(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "new", "v1", "modern":
		return "new"
	default:
		return "old"
	}
}

func preferredAppURL(theme string) string {
	if normalizeUITheme(theme) == "new" {
		return "/app-v1"
	}
	return "/app"
}
