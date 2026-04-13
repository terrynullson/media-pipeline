package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"media-pipeline/internal/domain/ports"
	"media-pipeline/internal/observability"
)

// MachineAPIHandler handles polling endpoints intended for n8n and other
// external automation tools. It has no dependency on HTML templates or
// business-logic services other than TranscriptViewService.
type MachineAPIHandler struct {
	transcriptViewUC TranscriptViewService
	logger           *slog.Logger
}

func NewMachineAPIHandler(transcriptViewUC TranscriptViewService, logger *slog.Logger) *MachineAPIHandler {
	return &MachineAPIHandler{transcriptViewUC: transcriptViewUC, logger: logger}
}

func (h *MachineAPIHandler) writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

// APIMediaStatus returns a lightweight status payload suitable for polling.
func (h *MachineAPIHandler) APIMediaStatus(w http.ResponseWriter, r *http.Request) {
	mediaID, err := mediaIDFromRequest(r)
	if err != nil {
		http.Error(w, "invalid media id", http.StatusBadRequest)
		return
	}

	result, err := h.transcriptViewUC.Load(r.Context(), mediaID)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		observability.LoggerFromContext(r.Context(), h.logger).Error(
			"machine api status load failed", "media_id", mediaID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, buildTranscriptAutomationStatus(result))
}

// APIMediaResult returns the full transcript result once processing is complete.
func (h *MachineAPIHandler) APIMediaResult(w http.ResponseWriter, r *http.Request) {
	mediaID, err := mediaIDFromRequest(r)
	if err != nil {
		http.Error(w, "invalid media id", http.StatusBadRequest)
		return
	}

	result, err := h.transcriptViewUC.Load(r.Context(), mediaID)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		observability.LoggerFromContext(r.Context(), h.logger).Error(
			"machine api result load failed", "media_id", mediaID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if !result.HasTranscript {
		http.Error(w, "transcript not ready", http.StatusConflict)
		return
	}

	h.writeJSON(w, http.StatusOK, apiMediaResultResponse{
		MediaID:    result.Media.ID,
		Name:       result.Media.OriginalName,
		Transcript: result.Transcript.FullText,
		Language:   result.Transcript.Language,
	})
}
