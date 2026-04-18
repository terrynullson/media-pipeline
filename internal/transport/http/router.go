package httptransport

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"media-pipeline/internal/transport/http/handlers"
)

func NewRouter(
	logger *slog.Logger,
	uploadHandler *handlers.UploadHandler,
	machineAPIHandler *handlers.MachineAPIHandler,
	triggerRuleHandler *handlers.TriggerRuleHandler,
	workerStatusHandler *handlers.WorkerStatusHandler,
	staticDir string,
	uploadsDir string,
	audioDir string,
	previewDir string,
	screenshotsDir string,
	mediaAccessToken string,
	requestTimeout time.Duration,
	uploadRateLimitPerMinute int64,
	frontendDirs ...string,
) http.Handler {
	r := chi.NewRouter()
	r.Use(RequestIDMiddleware(logger))
	r.Use(AccessLogMiddleware(logger))
	r.Use(RecoverMiddleware(logger))

	// Apply request timeout to all routes except /upload (large file transfers).
	timeout := RequestTimeoutMiddleware(requestTimeout)

	r.With(timeout).Get("/api/dashboard", uploadHandler.APIDashboard)
	r.With(timeout).Get("/api/media", uploadHandler.APIMediaList)
	r.With(timeout).Get("/api/jobs", uploadHandler.APIJobsList)
	r.With(timeout).Get("/api/media/{mediaID}", uploadHandler.APIMediaDetail)
	r.With(timeout).Get("/api/media/{mediaID}/transcript/export", uploadHandler.ExportTranscript)
	r.With(timeout).Post("/api/media/{mediaID}/retry", uploadHandler.RetryJob)
	r.With(timeout).Post("/api/media/bulk-delete", uploadHandler.BulkDeleteMedia)
	r.With(timeout).Get("/api/settings/transcription", uploadHandler.APITranscriptionSettings)
	r.With(timeout).Put("/api/settings/transcription", uploadHandler.APIUpdateTranscriptionSettings)
	r.With(timeout).Get("/api/settings/runtime", uploadHandler.APIRuntimeSettings)
	r.With(timeout).Put("/api/settings/runtime", uploadHandler.APIUpdateRuntimeSettings)
	r.With(timeout).Get("/api/ui-config", uploadHandler.APIUIConfig)
	r.With(timeout).Put("/api/ui-preference", uploadHandler.APIUpdateUITheme)
	r.With(timeout).Get("/api/trigger-rules", triggerRuleHandler.APITriggerRules)
	r.With(timeout).Post("/api/trigger-rules", triggerRuleHandler.APICreateTriggerRule)
	r.With(timeout).Post("/api/trigger-rules/preview", triggerRuleHandler.APIPreviewTriggerRule)
	r.With(timeout).Patch("/api/trigger-rules/{ruleID}", triggerRuleHandler.APIUpdateTriggerRule)
	r.With(timeout).Delete("/api/trigger-rules/{ruleID}", triggerRuleHandler.APIDeleteTriggerRule)

	mediaToken := MediaTokenMiddleware(mediaAccessToken)

	r.With(timeout, mediaToken).Get("/api/media/{mediaID}/status", machineAPIHandler.APIMediaStatus)
	r.With(timeout, mediaToken).Get("/api/media/{mediaID}/result", machineAPIHandler.APIMediaResult)
	r.With(timeout).Get("/api/worker/status", workerStatusHandler.APIWorkerStatus)

	r.With(timeout).Get("/workspace", uploadHandler.Workspace)
	uploadRateLimit := UploadRateLimitMiddleware(uploadRateLimitPerMinute)
	r.With(uploadRateLimit).Post("/upload", uploadHandler.Upload) // no timeout — large file uploads
	r.With(timeout).Get("/media/statuses", uploadHandler.MediaStatuses)
	r.With(timeout).Get("/media/{mediaID}/transcript", uploadHandler.Transcript)
	r.With(timeout).Post("/media/{mediaID}/summary", uploadHandler.RequestSummary)
	r.With(timeout).Post("/media/{mediaID}/delete", uploadHandler.DeleteMedia)
	r.With(timeout).Post("/trigger-rules", triggerRuleHandler.CreateTriggerRule)
	r.With(timeout).Post("/trigger-rules/{ruleID}/toggle", triggerRuleHandler.ToggleTriggerRule)
	r.With(timeout).Post("/trigger-rules/{ruleID}/delete", triggerRuleHandler.DeleteTriggerRule)
	r.With(timeout).Post("/settings/transcription", uploadHandler.SaveTranscriptionSettings)
	r.With(timeout).Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	r.Handle("/metrics", promhttp.Handler())

	fs := http.FileServer(http.Dir(staticDir))
	r.Handle("/static/*", http.StripPrefix("/static/", fs))

	uploadFS := http.FileServer(http.Dir(uploadsDir))
	r.With(mediaToken).Handle("/media-source/*", http.StripPrefix("/media-source/", uploadFS))

	audioFS := http.FileServer(http.Dir(audioDir))
	r.With(mediaToken).Handle("/media-audio/*", http.StripPrefix("/media-audio/", audioFS))

	previewFS := http.FileServer(http.Dir(previewDir))
	r.With(mediaToken).Handle("/media-preview/*", http.StripPrefix("/media-preview/", previewFS))

	screenshotFS := http.FileServer(http.Dir(screenshotsDir))
	r.With(mediaToken).Handle("/media-screenshots/*", http.StripPrefix("/media-screenshots/", screenshotFS))

	// Frontend v1 - /app-v1
	frontendDir := ""
	if len(frontendDirs) > 0 {
		frontendDir = frontendDirs[0]
	}
	if strings.TrimSpace(frontendDir) != "" {
		r.With(timeout).Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/app-v1", http.StatusSeeOther)
		})
		spaHandler := newSPAHandler(frontendDir, "/app-v1")
		r.Get("/app-v1", spaHandler)
		r.Get("/app-v1/*", spaHandler)
	} else {
		r.With(timeout).Get("/", uploadHandler.Index)
	}

	return r
}

func newSPAHandler(frontendDir string, prefix string) http.HandlerFunc {
	indexPath := filepath.Join(frontendDir, "index.html")

	return func(w http.ResponseWriter, r *http.Request) {
		requestPath := strings.TrimPrefix(r.URL.Path, prefix)
		requestPath = strings.TrimPrefix(requestPath, "/")
		if requestPath == "" {
			http.ServeFile(w, r, indexPath)
			return
		}

		candidate := filepath.Join(frontendDir, filepath.FromSlash(requestPath))
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			http.ServeFile(w, r, candidate)
			return
		}

		http.ServeFile(w, r, indexPath)
	}
}
