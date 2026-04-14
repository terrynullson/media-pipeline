package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	mediaapp "media-pipeline/internal/app/media"
	transcriptionapp "media-pipeline/internal/app/transcription"
	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/media"
	"media-pipeline/internal/domain/transcript"
	"media-pipeline/internal/domain/transcription"
	domaintrigger "media-pipeline/internal/domain/trigger"
	"media-pipeline/internal/observability"
)

type TranscriptPageViewData struct {
	PageNotice          string
	PageNoticeTone      string
	MediaID             int64
	MediaName           string
	SizeHuman           string
	CreatedAtUTC        string
	StatusLabel         string
	StatusTone          string
	PipelineStageLabel  string
	PipelineStageValue  int
	PipelineStageTotal  int
	PipelineCurrentStep string
	PipelineFailedStep  string
	PipelineError       string
	PipelineErrorHint   string
	PipelineSteps       []PipelineStepView
	DeleteURL           string
	Player              TranscriptPlayerView
	HasTranscript       bool
	Settings            []TranscriptSettingItem
	SettingsWarnings    []string
	SettingsUnavailable bool
	RuntimePolicy       TranscriptRuntimePolicyView
	RuntimeSnapshot     []TranscriptSettingItem
	FullTextParagraphs  []string
	Segments            []TranscriptSegmentView
	TriggerMatches      []TriggerEventView
	TriggerStatusLabel  string
	TriggerStatusTone   string
	TriggerNotice       string
	TriggerNoticeTone   string
	HasSummary          bool
	SummaryText         string
	SummaryHighlights   []string
	SummaryProvider     string
	SummaryUpdatedAtUTC string
	SummaryStatusLabel  string
	SummaryStatusTone   string
	SummaryNotice       string
	SummaryNoticeTone   string
	SummaryStep         PipelineStepView
	HasSummaryStep      bool
	ShowSummaryAction   bool
	SummaryActionLabel  string
	SummaryActionURL    string
}

type TranscriptPlayerView struct {
	HasMediaPlayer            bool   `json:"hasMediaPlayer"`
	IsAudioOnly               bool   `json:"isAudioOnly"`
	HasVideoPlayer            bool   `json:"hasVideoPlayer"`
	VideoSourceURL            string `json:"videoSourceURL"`
	VideoSourceType           string `json:"videoSourceType"`
	HasAudioPlayer            bool   `json:"hasAudioPlayer"`
	AudioPlayerURL            string `json:"audioPlayerURL"`
	AudioPlayerType           string `json:"audioPlayerType"`
	HasSecondaryAudioFallback bool   `json:"hasSecondaryAudioFallback"`
	AudioFallbackURL          string `json:"audioFallbackURL"`
	AudioFallbackType         string `json:"audioFallbackType"`
	UseAudioFallbackAsPrimary bool   `json:"useAudioFallbackAsPrimary"`
	PreviewStatusLabel        string `json:"previewStatusLabel"`
	PreviewStatusTone         string `json:"previewStatusTone"`
	PreviewNotice             string `json:"previewNotice"`
	PreviewNoticeTone         string `json:"previewNoticeTone"`
	PlayerFallbackText        string `json:"playerFallbackText"`
}

type TranscriptSettingItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type TranscriptRuntimePolicyView struct {
	Visible          bool     `json:"visible"`
	Title            string   `json:"title"`
	Tone             string   `json:"tone"`
	Summary          string   `json:"summary"`
	DurationLabel    string   `json:"durationLabel"`
	DurationClass    string   `json:"durationClass"`
	EffectiveTimeout string   `json:"effectiveTimeout"`
	Warnings         []string `json:"warnings"`
}

type TranscriptSegmentView struct {
	Index         int     `json:"index"`
	StartSec      float64 `json:"startSec"`
	EndSec        float64 `json:"endSec"`
	StartLabel    string  `json:"startLabel"`
	EndLabel      string  `json:"endLabel"`
	Text          string  `json:"text"`
	Confidence    string  `json:"confidence"`
	HasConfidence bool    `json:"hasConfidence"`
}

type TriggerEventView struct {
	Category          string  `json:"category"`
	RuleName          string  `json:"ruleName"`
	MatchedPhrase     string  `json:"matchedPhrase"`
	SeekSec           float64 `json:"seekSec"`
	Timestamp         string  `json:"timestamp"`
	SegmentText       string  `json:"segmentText"`
	ContextText       string  `json:"contextText"`
	HasScreenshot     bool    `json:"hasScreenshot"`
	ScreenshotSeekSec float64 `json:"screenshotSeekSec"`
	ScreenshotURL     string  `json:"screenshotURL"`
	ScreenshotAlt     string  `json:"screenshotAlt"`
	ScreenshotW       int     `json:"screenshotW"`
	ScreenshotH       int     `json:"screenshotH"`
	Placeholder       string  `json:"placeholder"`
}

func (h *UploadHandler) Transcript(w http.ResponseWriter, r *http.Request) {
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

		observability.LoggerFromContext(r.Context(), h.logger).Error(
			"load transcript view failed",
			slog.Int64("media_id", mediaID),
			slog.Any("error", err),
		)
		http.Error(w, "не удалось загрузить страницу расшифровки", http.StatusInternalServerError)
		return
	}

	jobs := make([]job.Job, 0, 5)
	if result.ExtractAudioJob != nil {
		jobs = append(jobs, *result.ExtractAudioJob)
	}
	if result.TranscribeJob != nil {
		jobs = append(jobs, *result.TranscribeJob)
	}
	if result.AnalyzeJob != nil {
		jobs = append(jobs, *result.AnalyzeJob)
	}
	if result.ScreenshotJob != nil {
		jobs = append(jobs, *result.ScreenshotJob)
	}
	if result.SummaryJob != nil {
		jobs = append(jobs, *result.SummaryJob)
	}
	pipelineView := buildMediaPipelineView(result.Media, jobs)
	playerView := buildTranscriptPlayerView(result)
	data := TranscriptPageViewData{
		PageNotice:          transcriptFlashMessage(r.URL.Query().Get("summary_status")),
		PageNoticeTone:      transcriptFlashTone(r.URL.Query().Get("summary_status")),
		MediaID:             result.Media.ID,
		MediaName:           result.Media.OriginalName,
		SizeHuman:           HumanSize(result.Media.SizeBytes),
		CreatedAtUTC:        FormatDateTimeUTC(result.Media.CreatedAtUTC),
		StatusLabel:         pipelineView.StatusLabel,
		StatusTone:          pipelineView.StatusTone,
		PipelineStageLabel:  pipelineView.StageLabel,
		PipelineStageValue:  pipelineView.StageValue,
		PipelineStageTotal:  pipelineView.StageTotal,
		PipelineCurrentStep: pipelineView.CurrentStage,
		PipelineFailedStep:  pipelineView.FailedStage,
		PipelineError:       pipelineView.ErrorSummary,
		PipelineErrorHint:   pipelineView.ErrorLocation,
		PipelineSteps:       pipelineView.Steps,
		DeleteURL:           fmt.Sprintf("/media/%d/delete", result.Media.ID),
		Player:              playerView,
		HasTranscript:       result.HasTranscript,
		Settings:            buildTranscriptSettings(result.Settings),
		SettingsWarnings:    buildTranscriptSettingsWarnings(result.Settings),
		SettingsUnavailable: result.SettingsUnavailable,
		RuntimePolicy:       buildTranscriptRuntimePolicyView(result),
		RuntimeSnapshot:     buildRuntimeSnapshotItems(result.Media.RuntimeSnapshotJSON),
	}
	if result.HasTranscript {
		data.FullTextParagraphs = buildTranscriptParagraphs(result.Transcript.Segments, result.Transcript.FullText)
		data.Segments = buildTranscriptSegments(result.Transcript.Segments)
	}
	if result.HasSummary {
		data.HasSummary = true
		data.SummaryText = strings.TrimSpace(result.Summary.SummaryText)
		data.SummaryHighlights = append([]string(nil), result.Summary.Highlights...)
		data.SummaryProvider = fallbackValue(result.Summary.Provider, "не указан")
		data.SummaryUpdatedAtUTC = FormatDateTimeUTC(result.Summary.UpdatedAtUTC)
	}
	data.TriggerMatches = buildTriggerEventViews(result.TriggerEvents, result.TriggerScreenshots, result.Media, result.ScreenshotJob)
	data.TriggerStatusLabel, data.TriggerStatusTone, data.TriggerNotice, data.TriggerNoticeTone = describeTriggerAnalysis(result.AnalyzeJob, len(data.TriggerMatches))
	data.SummaryStatusLabel, data.SummaryStatusTone, data.SummaryNotice, data.SummaryNoticeTone = describeSummaryState(result.SummaryJob, result.HasSummary)
	if result.SummaryJob != nil {
		data.SummaryStep = buildSummaryStepView(result.SummaryJob)
		data.HasSummaryStep = true
	}
	data.ShowSummaryAction, data.SummaryActionLabel = summaryActionState(result.SummaryJob, result.HasTranscript, result.HasSummary)
	data.SummaryActionURL = fmt.Sprintf("/media/%d/summary", result.Media.ID)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if execErr := h.tmpl.ExecuteTemplate(w, "transcript.html", data); execErr != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("render transcript template failed", slog.Any("error", execErr))
		http.Error(w, "не удалось отрисовать страницу расшифровки", http.StatusInternalServerError)
	}
}

func (h *UploadHandler) RequestSummary(w http.ResponseWriter, r *http.Request) {
	mediaID, err := mediaIDFromRequest(r)
	if err != nil {
		http.Error(w, "некорректный media id", http.StatusBadRequest)
		return
	}

	result, err := h.summaryRequestUC.Request(r.Context(), mediaID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			http.NotFound(w, r)
			return
		case errors.Is(err, mediaapp.ErrSummaryTranscriptNotReady):
			http.Redirect(w, r, fmt.Sprintf("/media/%d/transcript?summary_status=not_ready", mediaID), http.StatusSeeOther)
			return
		default:
			observability.LoggerFromContext(r.Context(), h.logger).Error(
				"request summary failed",
				slog.Int64("media_id", mediaID),
				slog.Any("error", err),
			)
			http.Error(w, "не удалось поставить саммари в очередь", http.StatusInternalServerError)
			return
		}
	}

	status := "requested"
	if result.AlreadyInFlight {
		status = "in_progress"
	}

	http.Redirect(w, r, fmt.Sprintf("/media/%d/transcript?summary_status=%s", mediaID, status), http.StatusSeeOther)
}

// RetryJob requeues the latest failed job for the given media item so the
// worker picks it up again on the next poll cycle.
func (h *UploadHandler) RetryJob(w http.ResponseWriter, r *http.Request) {
	mediaID, err := mediaIDFromRequest(r)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "некорректный media id"})
		return
	}

	result, err := h.retryJobUC.Retry(r.Context(), mediaID)
	if err != nil {
		if errors.Is(err, mediaapp.ErrNoFailedJobs) {
			h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "нет упавших задач для повтора"})
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "медиафайл не найден"})
			return
		}
		observability.LoggerFromContext(r.Context(), h.logger).Error(
			"retry job failed",
			slog.Int64("media_id", mediaID),
			slog.Any("error", err),
		)
		h.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "не удалось поставить задачу в очередь повторно"})
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"status": "requeued",
		"jobId":  result.JobID,
	})
}

func (h *UploadHandler) DeleteMedia(w http.ResponseWriter, r *http.Request) {
	mediaID, err := mediaIDFromRequest(r)
	if err != nil {
		http.Error(w, "некорректный media id", http.StatusBadRequest)
		return
	}

	result, err := h.deleteMediaUC.Delete(r.Context(), mediaID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if wantsJSON(r) {
				h.writeJSON(w, http.StatusNotFound, map[string]any{
					"status":  "error",
					"message": "Файл не найден.",
				})
				return
			}
			http.NotFound(w, r)
			return
		}

		observability.LoggerFromContext(r.Context(), h.logger).Error(
			"delete media failed",
			slog.Int64("media_id", mediaID),
			slog.Any("error", err),
		)
		if wantsJSON(r) {
			h.writeJSON(w, http.StatusInternalServerError, map[string]any{
				"status":  "error",
				"message": "Не удалось удалить файл.",
			})
			return
		}
		http.Error(w, "не удалось удалить файл", http.StatusInternalServerError)
		return
	}

	message := "Файл удалён."
	if len(result.CleanupWarnings) > 0 {
		message = "Файл удалён, но часть файлов не получилось очистить автоматически."
	}
	if wantsJSON(r) {
		h.writeJSON(w, http.StatusOK, map[string]any{
			"status":   "deleted",
			"mediaId":  result.MediaID,
			"message":  message,
			"warnings": result.CleanupWarnings,
		})
		return
	}

	http.Redirect(w, r, "/?status=deleted", http.StatusSeeOther)
}

func mediaIDFromRequest(r *http.Request) (int64, error) {
	raw := strings.TrimSpace(chi.URLParam(r, "mediaID"))
	if raw == "" {
		return 0, fmt.Errorf("media id is required")
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid media id %q", raw)
	}

	return value, nil
}

// ExportTranscript serves the transcript in one of three downloadable formats:
//
//	?format=srt  — SubRip subtitle file
//	?format=txt  — plain text (full_text, newline-separated segments)
//	?format=json — JSON matching the /result API response shape
func (h *UploadHandler) ExportTranscript(w http.ResponseWriter, r *http.Request) {
	mediaID, err := mediaIDFromRequest(r)
	if err != nil {
		http.Error(w, "некорректный media id", http.StatusBadRequest)
		return
	}

	format := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("format")))
	if format == "" {
		format = "txt"
	}

	result, err := h.transcriptViewUC.Load(r.Context(), mediaID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "транскрипт не найден"})
			return
		}
		observability.LoggerFromContext(r.Context(), h.logger).Error(
			"export transcript load failed",
			slog.Int64("media_id", mediaID),
			slog.Any("error", err),
		)
		http.Error(w, "не удалось загрузить транскрипт", http.StatusInternalServerError)
		return
	}

	if !result.HasTranscript {
		h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "транскрипт не найден"})
		return
	}

	baseName := strings.TrimSuffix(result.Media.OriginalName, filepath.Ext(result.Media.OriginalName))
	baseName = sanitizeFilename(baseName)

	switch format {
	case "srt":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.srt"`, baseName))
		_, _ = fmt.Fprint(w, formatSRT(result.Transcript.Segments))

	case "txt":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.txt"`, baseName))
		_, _ = fmt.Fprint(w, formatTXT(result.Transcript))

	case "json":
		type segmentJSON struct {
			Index     int     `json:"index"`
			StartSec  float64 `json:"startSec"`
			EndSec    float64 `json:"endSec"`
			Text      string  `json:"text"`
		}
		segs := make([]segmentJSON, 0, len(result.Transcript.Segments))
		for i, s := range result.Transcript.Segments {
			segs = append(segs, segmentJSON{Index: i, StartSec: s.StartSec, EndSec: s.EndSec, Text: s.Text})
		}
		payload := map[string]any{
			"mediaId":  result.Media.ID,
			"name":     result.Media.OriginalName,
			"language": result.Transcript.Language,
			"fullText": result.Transcript.FullText,
			"segments": segs,
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s_transcript.json"`, baseName))
		_ = json.NewEncoder(w).Encode(payload)

	default:
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "неизвестный формат; допустимые значения: srt, txt, json"})
	}
}

// formatSRT converts transcript segments to SubRip (.srt) text.
func formatSRT(segments []transcript.Segment) string {
	if len(segments) == 0 {
		return ""
	}

	var b strings.Builder
	for i, s := range segments {
		text := strings.TrimSpace(s.Text)
		if text == "" {
			continue
		}
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n",
			i+1,
			FormatSRTTimestamp(s.StartSec),
			FormatSRTTimestamp(s.EndSec),
			text,
		)
	}
	return b.String()
}

// formatTXT returns plain text: each segment on its own line.
// Falls back to FullText when no segments are available.
func formatTXT(t transcript.Transcript) string {
	if len(t.Segments) == 0 {
		return strings.TrimSpace(t.FullText)
	}

	var b strings.Builder
	for _, s := range t.Segments {
		text := strings.TrimSpace(s.Text)
		if text == "" {
			continue
		}
		b.WriteString(text)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// sanitizeFilename removes or replaces characters that are unsafe in filenames.
func sanitizeFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r == '/' || r == '\\' || r == ':' || r == '*' ||
			r == '?' || r == '"' || r == '<' || r == '>' || r == '|':
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	result := strings.TrimSpace(b.String())
	if result == "" {
		return "transcript"
	}
	return result
}

func buildTranscriptSettings(settings *transcription.Settings) []TranscriptSettingItem {
	if settings == nil {
		return nil
	}

	return []TranscriptSettingItem{
		{Label: "Бэкенд", Value: string(settings.Backend)},
		{Label: "Модель", Value: settings.ModelName},
		{Label: "Устройство", Value: settings.Device},
		{Label: "Тип вычислений", Value: settings.ComputeType},
		{Label: "Язык", Value: fallbackValue(settings.Language, "авто")},
		{Label: "Лучей поиска", Value: strconv.Itoa(settings.BeamSize)},
		{Label: "VAD", Value: boolLabel(settings.VADEnabled)},
	}
}

func buildTranscriptSettingsWarnings(settings *transcription.Settings) []string {
	if settings == nil {
		return nil
	}
	return transcription.BuildRuntimeSettingsWarnings(*settings)
}

func buildTranscriptRuntimePolicyView(result mediaapp.TranscriptViewResult) TranscriptRuntimePolicyView {
	if result.Settings == nil {
		return TranscriptRuntimePolicyView{}
	}

	if !result.RuntimePolicyReady || result.RuntimePolicy == nil {
		switch {
		case strings.TrimSpace(result.Media.ExtractedAudioPath) == "":
			return TranscriptRuntimePolicyView{
				Visible: true,
				Title:   "Оценка времени запуска",
				Tone:    "warning",
				Summary: "Оценка времени появится после извлечения аудио.",
			}
		default:
			return TranscriptRuntimePolicyView{
				Visible: true,
				Title:   "Оценка времени запуска",
				Tone:    "warning",
				Summary: "Не удалось заранее оценить время распознавания для этого файла.",
			}
		}
	}

	policy := result.RuntimePolicy
	view := TranscriptRuntimePolicyView{
		Visible:          true,
		Title:            "Оценка времени запуска",
		Tone:             "warning",
		DurationLabel:    transcriptionapp.FormatRuntimeDurationRU(policy.MediaDuration),
		DurationClass:    capitalizeFirst(transcriptionapp.DurationClassLabelRU(policy.DurationClass)),
		EffectiveTimeout: transcriptionapp.FormatRuntimeDurationRU(policy.EffectiveTimeout),
		Warnings:         append([]string(nil), policy.Warnings...),
	}
	switch {
	case policy.Blocked:
		view.Tone = "error"
		view.Summary = "Эта конфигурация для данного файла слишком тяжёлая. Worker не запускает её автоматически."
	case policy.HasAdaptiveTimeout():
		view.Tone = "warning"
		view.Summary = fmt.Sprintf("Для этого файла лимит распознавания автоматически увеличен до %s.", view.EffectiveTimeout)
	default:
		view.Tone = "success"
		view.Summary = fmt.Sprintf("Для этого файла достаточно стандартного лимита %s.", transcriptionapp.FormatRuntimeDurationRU(policy.BaseTimeout))
	}

	return view
}

func buildTranscriptSegments(items []transcript.Segment) []TranscriptSegmentView {
	segments := make([]TranscriptSegmentView, 0, len(items))
	for index, item := range items {
		segment := TranscriptSegmentView{
			Index:      index,
			StartSec:   item.StartSec,
			EndSec:     item.EndSec,
			StartLabel: FormatTimestamp(item.StartSec),
			EndLabel:   FormatTimestamp(item.EndSec),
			Text:       strings.TrimSpace(item.Text),
		}
		if item.Confidence != nil {
			segment.HasConfidence = true
			segment.Confidence = fmt.Sprintf("%.2f", *item.Confidence)
		}
		segments = append(segments, segment)
	}

	return segments
}

func buildTriggerEventViews(
	items []domaintrigger.Event,
	screenshots map[int64]domaintrigger.Screenshot,
	mediaItem media.Media,
	screenshotJob *job.Job,
) []TriggerEventView {
	views := make([]TriggerEventView, 0, len(items))
	for _, item := range items {
		contextText := strings.TrimSpace(item.ContextText)
		segmentText := strings.TrimSpace(item.SegmentText)
		if contextText == segmentText {
			contextText = ""
		}

		view := TriggerEventView{
			Category:          item.Category,
			RuleName:          item.RuleName,
			MatchedPhrase:     item.MatchedText,
			SeekSec:           item.StartSec,
			Timestamp:         FormatTimestamp(item.StartSec),
			SegmentText:       segmentText,
			ContextText:       contextText,
			ScreenshotSeekSec: item.StartSec,
			ScreenshotAlt:     fmt.Sprintf("Скриншот для %s на %s", item.MatchedText, FormatTimestamp(item.StartSec)),
			Placeholder:       describeScreenshotPlaceholder(mediaItem, screenshotJob),
		}
		if screenshot, ok := screenshots[item.ID]; ok {
			view.HasScreenshot = true
			view.ScreenshotSeekSec = screenshot.TimestampSec
			view.ScreenshotURL = "/media-screenshots/" + screenshot.ImagePath
			view.ScreenshotW = screenshot.Width
			view.ScreenshotH = screenshot.Height
			view.Placeholder = ""
		}

		views = append(views, view)
	}

	return views
}

func buildMediaSourceURL(relativePath string) string {
	relativePath = strings.TrimSpace(relativePath)
	if relativePath == "" {
		return ""
	}

	return "/media-source/" + filepath.ToSlash(relativePath)
}

func buildMediaAudioURL(relativePath string) string {
	relativePath = strings.TrimSpace(relativePath)
	if relativePath == "" {
		return ""
	}

	return "/media-audio/" + filepath.ToSlash(relativePath)
}

func buildMediaPreviewURL(relativePath string) string {
	relativePath = strings.TrimSpace(relativePath)
	if relativePath == "" {
		return ""
	}

	return "/media-preview/" + filepath.ToSlash(relativePath)
}

func buildTranscriptPlayerView(result mediaapp.TranscriptViewResult) TranscriptPlayerView {
	view := TranscriptPlayerView{
		IsAudioOnly:       result.Media.IsAudioOnly(),
		AudioFallbackType: "audio/wav",
	}

	if result.Media.IsAudioOnly() {
		view.HasAudioPlayer = result.MediaSourceReady
		view.AudioPlayerURL = buildMediaSourceURL(result.MediaSourcePath)
		view.AudioPlayerType = strings.TrimSpace(result.Media.MIMEType)
		view.HasMediaPlayer = view.HasAudioPlayer
		if !view.HasMediaPlayer {
			view.PlayerFallbackText = "Аудиофайл сейчас недоступен для встроенного проигрывателя, но страница остаётся доступной."
		}
		return view
	}

	view.HasVideoPlayer = result.PreviewSourceReady
	view.VideoSourceURL = buildMediaPreviewURL(result.PreviewSourcePath)
	view.VideoSourceType = fallbackValue(strings.TrimSpace(result.Media.PreviewVideoMIMEType), "video/mp4")
	view.PreviewStatusLabel, view.PreviewStatusTone, view.PreviewNotice, view.PreviewNoticeTone = describePreviewState(result.PreviewJob, result.PreviewSourceReady)

	if result.PreviewSourceReady {
		view.HasMediaPlayer = true
		if result.AudioSourceReady {
			view.HasSecondaryAudioFallback = true
			view.AudioFallbackURL = buildMediaAudioURL(result.AudioSourcePath)
		}
		return view
	}

	if result.AudioSourceReady {
		view.HasMediaPlayer = true
		view.HasAudioPlayer = true
		view.UseAudioFallbackAsPrimary = true
		view.AudioPlayerURL = buildMediaAudioURL(result.AudioSourcePath)
		return view
	}

	view.PlayerFallbackText = describeVideoPlayerFallback(result)
	return view
}

func describePreviewState(currentJob *job.Job, previewReady bool) (label string, tone string, notice string, noticeTone string) {
	if previewReady {
		return "Готово", "success", "", ""
	}
	if currentJob == nil {
		return "Недоступно", "neutral", "Browser-safe preview для этого видео ещё не подготовлен.", "neutral"
	}

	switch currentJob.Status {
	case job.StatusPending:
		return "В очереди", "ready", "Browser-safe preview поставлен в очередь. Пока можно использовать audio fallback, если он уже готов.", "neutral"
	case job.StatusRunning:
		return "В работе", "running", "Browser-safe preview сейчас готовится. Пока можно использовать audio fallback, если он уже готов.", "neutral"
	case job.StatusFailed:
		message := "Не удалось подготовить browser-safe preview видео."
		if current := userFacingJobError(currentJob); current != "" {
			message = current
		}
		return "Ошибка", "error", message, "error"
	case job.StatusDone:
		return "Недоступно", "warning", "Preview job завершился, но preview-файл не найден на диске. Можно использовать audio fallback.", "warning"
	default:
		return "Недоступно", "neutral", "Browser-safe preview сейчас недоступен.", "neutral"
	}
}

func describeVideoPlayerFallback(result mediaapp.TranscriptViewResult) string {
	if result.PreviewJob != nil && result.PreviewJob.Status == job.StatusFailed {
		return "Browser-safe preview недоступен, а извлечённая аудиодорожка тоже пока не готова."
	}
	if result.PreviewJob != nil && (result.PreviewJob.Status == job.StatusPending || result.PreviewJob.Status == job.StatusRunning) {
		return "Browser-safe preview ещё готовится. Проигрыватель появится автоматически после завершения задачи."
	}
	return "Browser-safe preview сейчас недоступен, поэтому встроенный video player временно не показывается."
}

func describeMediaPlayerFallback(mediaItem media.Media, sourceReady bool) string {
	if sourceReady {
		return ""
	}
	if strings.TrimSpace(mediaItem.StoragePath) == "" {
		return "Исходный медиафайл для встроенного плеера не найден."
	}
	if mediaItem.IsAudioOnly() {
		return "Аудиофайл сейчас недоступен для встроенного проигрывателя, но расшифровка и таймлайн остаются доступными."
	}
	return "Видеофайл сейчас недоступен для встроенного проигрывателя, но расшифровка и таймлайн остаются доступными."
}

func describeScreenshotPlaceholder(mediaItem media.Media, currentJob *job.Job) string {
	if mediaItem.IsAudioOnly() {
		return "Для аудио скриншоты не создаются."
	}
	if currentJob == nil {
		return "Скриншоты ещё не запускались."
	}

	switch currentJob.Status {
	case job.StatusPending:
		return "Скриншот в очереди."
	case job.StatusRunning:
		return "Скриншот сейчас создаётся."
	case job.StatusFailed:
		if message := userFacingJobError(currentJob); message != "" {
			return message
		}
		return "Не удалось создать скриншот."
	case job.StatusDone:
		return "Для этого триггера скриншот не найден."
	default:
		return "Скриншот недоступен."
	}
}

func describeTriggerAnalysis(currentJob *job.Job, triggerCount int) (label string, tone string, notice string, noticeTone string) {
	if currentJob == nil {
		if triggerCount > 0 {
			return "Готово", "success", "", ""
		}
		return "Не запускалось", "neutral", "Анализ триггеров ещё не запускался.", "neutral"
	}

	switch currentJob.Status {
	case job.StatusPending:
		return "В очереди", "ready", "Анализ триггеров поставлен в очередь и будет выполнен worker-процессом.", "neutral"
	case job.StatusRunning:
		return "В работе", "running", "Анализ триггеров выполняется прямо сейчас.", "neutral"
	case job.StatusFailed:
		message := "Анализ триггеров завершился ошибкой."
		if current := userFacingJobError(currentJob); current != "" {
			message = current
		}
		return "Ошибка", "error", message, "error"
	case job.StatusDone:
		if triggerCount == 0 {
			return "Готово", "success", "Для этой расшифровки триггеры не найдены.", "neutral"
		}
		return "Готово", "success", "", ""
	default:
		return string(currentJob.Status), "neutral", "", ""
	}
}

func describeSummaryState(currentJob *job.Job, hasSummary bool) (label string, tone string, notice string, noticeTone string) {
	if currentJob == nil {
		if hasSummary {
			return "Готово", "success", "", ""
		}
		return "Не запускалось", "neutral", "Саммари создаётся только по вашему запросу.", "neutral"
	}

	switch currentJob.Status {
	case job.StatusPending:
		return "В очереди", "ready", "Саммари поставлено в очередь.", "neutral"
	case job.StatusRunning:
		return "В работе", "running", "Worker сейчас собирает саммари.", "neutral"
	case job.StatusFailed:
		message := "Не удалось создать саммари."
		if current := userFacingJobError(currentJob); current != "" {
			message = current
		}
		return "Ошибка", "error", message, "error"
	case job.StatusDone:
		if hasSummary {
			return "Готово", "success", "Саммари сохранено и доступно ниже.", "neutral"
		}
		return "Готово", "success", "Задача завершилась, но саммари не найдено.", "neutral"
	default:
		return string(currentJob.Status), "neutral", "", ""
	}
}

func summaryActionState(currentJob *job.Job, hasTranscript bool, hasSummary bool) (bool, string) {
	if !hasTranscript {
		return false, ""
	}
	if currentJob != nil && (currentJob.Status == job.StatusPending || currentJob.Status == job.StatusRunning) {
		return false, ""
	}
	if hasSummary {
		return true, "Сделать заново"
	}
	return true, "Сделать саммари"
}

func transcriptFlashMessage(status string) string {
	switch strings.TrimSpace(status) {
	case "requested":
		return "Задача на саммари поставлена в очередь."
	case "in_progress":
		return "Саммари уже создаётся. Дождитесь завершения."
	case "not_ready":
		return "Саммари можно запустить только после готовой расшифровки."
	default:
		return ""
	}
}

func transcriptFlashTone(status string) string {
	switch strings.TrimSpace(status) {
	case "not_ready":
		return "error"
	case "requested", "in_progress":
		return "success"
	default:
		return ""
	}
}

func buildTranscriptParagraphs(items []transcript.Segment, fullText string) []string {
	if len(items) == 0 {
		return splitTranscriptText(fullText)
	}

	paragraphs := make([]string, 0)
	var builder strings.Builder
	segmentCount := 0
	lastEnd := 0.0

	flush := func() {
		text := strings.TrimSpace(builder.String())
		if text == "" {
			builder.Reset()
			segmentCount = 0
			return
		}
		paragraphs = append(paragraphs, text)
		builder.Reset()
		segmentCount = 0
	}

	for _, item := range items {
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		if builder.Len() > 0 && lastEnd > 0 && item.StartSec-lastEnd >= 1.5 {
			flush()
		}
		if builder.Len() > 0 {
			builder.WriteByte(' ')
		}
		builder.WriteString(text)
		segmentCount++

		shouldFlush := false
		if segmentCount >= 4 {
			shouldFlush = true
		}
		if builder.Len() >= 420 {
			shouldFlush = true
		}
		if strings.HasSuffix(text, ".") || strings.HasSuffix(text, "!") || strings.HasSuffix(text, "?") {
			shouldFlush = shouldFlush || segmentCount >= 3
		}

		lastEnd = item.EndSec
		if shouldFlush {
			flush()
		}
	}

	flush()
	if len(paragraphs) > 0 {
		return paragraphs
	}

	return splitTranscriptText(fullText)
}

func splitTranscriptText(fullText string) []string {
	trimmed := strings.TrimSpace(fullText)
	if trimmed == "" {
		return nil
	}

	trimmed = strings.Join(strings.Fields(trimmed), " ")
	paragraphs := make([]string, 0)
	for len(trimmed) > 0 {
		if len(trimmed) <= 420 {
			paragraphs = append(paragraphs, trimmed)
			break
		}

		splitAt := strings.LastIndex(trimmed[:420], ". ")
		if splitAt < 160 {
			splitAt = strings.LastIndex(trimmed[:420], " ")
		}
		if splitAt < 0 {
			splitAt = 420
		}

		paragraphs = append(paragraphs, strings.TrimSpace(trimmed[:splitAt+1]))
		trimmed = strings.TrimSpace(trimmed[splitAt+1:])
	}

	return paragraphs
}

func fallbackValue(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func boolLabel(value bool) string {
	if value {
		return "включено"
	}
	return "выключено"
}

func capitalizeFirst(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	runes := []rune(value)
	first := strings.ToUpper(string(runes[0]))
	if len(runes) == 1 {
		return first
	}

	return first + string(runes[1:])
}

func buildSummaryStepView(currentJob *job.Job) PipelineStepView {
	step := describeJobBackedStep("Саммари", currentJob, time.Now().UTC())
	return PipelineStepView{
		Label:           step.label,
		StatusLabel:     step.statusLabel,
		Tone:            step.tone,
		TimingText:      step.timingText,
		StartedAtLabel:  step.startedAtLabel,
		FinishedAtLabel: step.finishedAtLabel,
		DurationLabel:   step.durationLabel,
		ProgressLabel:   step.progressLabel,
		ProgressPercent: step.progressPercent,
		ProgressVisible: step.progressVisible,
	}
}

func buildRuntimeSnapshotItems(raw string) []TranscriptSettingItem {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	snapshot, err := media.DecodeRuntimeSnapshot(raw)
	if err != nil {
		return nil
	}

	items := make([]TranscriptSettingItem, 0, 8)
	appendItem := func(label string, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		items = append(items, TranscriptSettingItem{Label: label, Value: value})
	}

	if !snapshot.CapturedAtUTC.IsZero() {
		appendItem("Собрано", FormatDateTimeUTC(snapshot.CapturedAtUTC))
	}
	appendItem("IP", snapshot.RequestIP)
	appendItem("User-Agent", snapshot.UserAgent)
	appendItem("Язык браузера", firstNonEmptyValue(snapshot.ClientLanguage, snapshot.AcceptLanguage))
	appendItem("Платформа", firstNonEmptyValue(snapshot.ClientPlatform, snapshot.ClientHintPlatform))
	if snapshot.HardwareConcurrency != nil {
		appendItem("Потоки браузера", strconv.Itoa(*snapshot.HardwareConcurrency))
	}
	if snapshot.DeviceMemoryGB != nil {
		appendItem("Память устройства", fmt.Sprintf("%.1f ГБ", *snapshot.DeviceMemoryGB))
	}
	if snapshot.TimezoneOffsetMinutes != nil {
		appendItem("Смещение часового пояса", fmt.Sprintf("%d мин", *snapshot.TimezoneOffsetMinutes))
	}

	return items
}

func firstNonEmptyValue(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}

	return ""
}
