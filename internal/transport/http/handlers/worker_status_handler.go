package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	mediaapp "media-pipeline/internal/app/media"
	"media-pipeline/internal/observability"
)

// WorkerStatusService loads the live worker state.
type WorkerStatusService interface {
	Load(ctx context.Context) (mediaapp.WorkerStatusResult, error)
}

// WorkerStatusHandler serves GET /api/worker/status.
type WorkerStatusHandler struct {
	svc    WorkerStatusService
	logger *slog.Logger
}

func NewWorkerStatusHandler(svc WorkerStatusService, logger *slog.Logger) *WorkerStatusHandler {
	return &WorkerStatusHandler{svc: svc, logger: logger}
}

func (h *WorkerStatusHandler) APIWorkerStatus(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.Load(r.Context())
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error(
			"worker status load failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := map[string]any{
		"workerHeartbeatAge": result.WorkerHeartbeatAge,
		"likelyAlive":        result.LikelyAlive,
		"currentJob":         result.CurrentJob,
		"queue": map[string]any{
			"pending": result.Queue.Pending,
			"byType":  result.Queue.ByType,
		},
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
