package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"media-pipeline/internal/domain/job"
	"media-pipeline/internal/domain/transcript"
	"media-pipeline/internal/domain/transcription"
	domaintrigger "media-pipeline/internal/domain/trigger"
	"media-pipeline/internal/observability"
)

type TranscriptPageViewData struct {
	MediaID             int64
	MediaName           string
	SizeHuman           string
	CreatedAtUTC        string
	StatusLabel         string
	StatusTone          string
	DeleteURL           string
	HasTranscript       bool
	Settings            []TranscriptSettingItem
	SettingsUnavailable bool
	FullTextParagraphs  []string
	Segments            []TranscriptSegmentView
	TriggerMatches      []TriggerEventView
	TriggerStatusLabel  string
	TriggerStatusTone   string
	TriggerNotice       string
	TriggerNoticeTone   string
}

type TranscriptSettingItem struct {
	Label string
	Value string
}

type TranscriptSegmentView struct {
	StartLabel    string
	EndLabel      string
	Text          string
	Confidence    string
	HasConfidence bool
}

type TriggerEventView struct {
	Category      string
	RuleName      string
	MatchedPhrase string
	Timestamp     string
	SegmentText   string
	ContextText   string
}

func (h *UploadHandler) Transcript(w http.ResponseWriter, r *http.Request) {
	mediaID, err := mediaIDFromRequest(r)
	if err != nil {
		http.Error(w, "invalid media id", http.StatusBadRequest)
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
		http.Error(w, "failed to load transcript view", http.StatusInternalServerError)
		return
	}

	statusLabel, statusTone, _, _, _ := describeMediaStatus(result.Media.Status)
	data := TranscriptPageViewData{
		MediaID:             result.Media.ID,
		MediaName:           result.Media.OriginalName,
		SizeHuman:           HumanSize(result.Media.SizeBytes),
		CreatedAtUTC:        FormatDateTimeUTC(result.Media.CreatedAtUTC),
		StatusLabel:         statusLabel,
		StatusTone:          statusTone,
		DeleteURL:           fmt.Sprintf("/media/%d/delete", result.Media.ID),
		HasTranscript:       result.HasTranscript,
		Settings:            buildTranscriptSettings(result.Settings),
		SettingsUnavailable: result.SettingsUnavailable,
	}
	if result.HasTranscript {
		data.FullTextParagraphs = buildTranscriptParagraphs(result.Transcript.Segments, result.Transcript.FullText)
		data.Segments = buildTranscriptSegments(result.Transcript.Segments)
	}
	data.TriggerMatches = buildTriggerEventViews(result.TriggerEvents)
	data.TriggerStatusLabel, data.TriggerStatusTone, data.TriggerNotice, data.TriggerNoticeTone = describeTriggerAnalysis(result.AnalyzeJob, len(data.TriggerMatches))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if execErr := h.tmpl.ExecuteTemplate(w, "transcript.html", data); execErr != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("render transcript template failed", slog.Any("error", execErr))
		http.Error(w, "failed to render transcript page", http.StatusInternalServerError)
	}
}

func (h *UploadHandler) DeleteMedia(w http.ResponseWriter, r *http.Request) {
	mediaID, err := mediaIDFromRequest(r)
	if err != nil {
		http.Error(w, "invalid media id", http.StatusBadRequest)
		return
	}

	result, err := h.deleteMediaUC.Delete(r.Context(), mediaID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if wantsJSON(r) {
				h.writeJSON(w, http.StatusNotFound, map[string]any{
					"status":  "error",
					"message": "Media item was not found.",
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
				"message": "Could not delete the media item.",
			})
			return
		}
		http.Error(w, "failed to delete media", http.StatusInternalServerError)
		return
	}

	message := "Media item was deleted."
	if len(result.CleanupWarnings) > 0 {
		message = "Media item was deleted. Some files could not be removed and were logged for follow-up."
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

func buildTranscriptSettings(settings *transcription.Settings) []TranscriptSettingItem {
	if settings == nil {
		return nil
	}

	return []TranscriptSettingItem{
		{Label: "Backend", Value: string(settings.Backend)},
		{Label: "Model", Value: settings.ModelName},
		{Label: "Device", Value: settings.Device},
		{Label: "Compute type", Value: settings.ComputeType},
		{Label: "Language", Value: fallbackValue(settings.Language, "auto")},
		{Label: "Beam size", Value: strconv.Itoa(settings.BeamSize)},
		{Label: "VAD", Value: boolLabel(settings.VADEnabled)},
	}
}

func buildTranscriptSegments(items []transcript.Segment) []TranscriptSegmentView {
	segments := make([]TranscriptSegmentView, 0, len(items))
	for _, item := range items {
		segment := TranscriptSegmentView{
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

func buildTriggerEventViews(items []domaintrigger.Event) []TriggerEventView {
	views := make([]TriggerEventView, 0, len(items))
	for _, item := range items {
		contextText := strings.TrimSpace(item.ContextText)
		segmentText := strings.TrimSpace(item.SegmentText)
		if contextText == segmentText {
			contextText = ""
		}

		views = append(views, TriggerEventView{
			Category:      item.Category,
			RuleName:      item.RuleName,
			MatchedPhrase: item.MatchedText,
			Timestamp:     FormatTimestamp(item.StartSec),
			SegmentText:   segmentText,
			ContextText:   contextText,
		})
	}

	return views
}

func describeTriggerAnalysis(currentJob *job.Job, triggerCount int) (label string, tone string, notice string, noticeTone string) {
	if currentJob == nil {
		if triggerCount > 0 {
			return "Ready", "success", "", ""
		}
		return "Not started", "neutral", "Trigger analysis has not started yet.", "neutral"
	}

	switch currentJob.Status {
	case job.StatusPending:
		return "Queued", "ready", "Trigger analysis is queued and will run in the worker.", "neutral"
	case job.StatusRunning:
		return "Running", "running", "Trigger analysis is running now.", "neutral"
	case job.StatusFailed:
		message := "Trigger analysis failed."
		if strings.TrimSpace(currentJob.ErrorMessage) != "" {
			message = currentJob.ErrorMessage
		}
		return "Failed", "error", message, "error"
	case job.StatusDone:
		if triggerCount == 0 {
			return "Done", "success", "No trigger matches were found for this transcript.", "neutral"
		}
		return "Done", "success", "", ""
	default:
		return string(currentJob.Status), "neutral", "", ""
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
		return "enabled"
	}
	return "disabled"
}
