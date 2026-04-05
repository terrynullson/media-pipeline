package httptransport

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"media-pipeline/internal/transport/http/handlers"
)

func NewRouter(logger *slog.Logger, uploadHandler *handlers.UploadHandler, staticDir string, uploadsDir string, audioDir string, previewDir string, screenshotsDir string, frontendDirs ...string) http.Handler {
	r := chi.NewRouter()
	r.Use(RequestIDMiddleware(logger))
	r.Use(AccessLogMiddleware(logger))
	r.Use(RecoverMiddleware(logger))

	r.Get("/api/dashboard", uploadHandler.APIDashboard)
	r.Get("/api/media", uploadHandler.APIMediaList)
	r.Get("/api/jobs", uploadHandler.APIJobsList)
	r.Get("/api/media/{mediaID}", uploadHandler.APIMediaDetail)
	r.Get("/api/settings/transcription", uploadHandler.APITranscriptionSettings)
	r.Put("/api/settings/transcription", uploadHandler.APIUpdateTranscriptionSettings)
	r.Get("/api/ui-config", uploadHandler.APIUIConfig)
	r.Put("/api/ui-preference", uploadHandler.APIUpdateUITheme)
	r.Get("/api/trigger-rules", uploadHandler.APITriggerRules)
	r.Post("/api/trigger-rules", uploadHandler.APICreateTriggerRule)
	r.Patch("/api/trigger-rules/{ruleID}", uploadHandler.APIUpdateTriggerRule)
	r.Delete("/api/trigger-rules/{ruleID}", uploadHandler.APIDeleteTriggerRule)

	r.Get("/", uploadHandler.Index)
	r.Get("/workspace", uploadHandler.Workspace)
	r.Post("/upload", uploadHandler.Upload)
	r.Get("/media/statuses", uploadHandler.MediaStatuses)
	r.Get("/media/{mediaID}/transcript", uploadHandler.Transcript)
	r.Post("/media/{mediaID}/summary", uploadHandler.RequestSummary)
	r.Post("/media/{mediaID}/delete", uploadHandler.DeleteMedia)
	r.Post("/trigger-rules", uploadHandler.CreateTriggerRule)
	r.Post("/trigger-rules/{ruleID}/toggle", uploadHandler.ToggleTriggerRule)
	r.Post("/trigger-rules/{ruleID}/delete", uploadHandler.DeleteTriggerRule)
	r.Post("/settings/transcription", uploadHandler.SaveTranscriptionSettings)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	fs := http.FileServer(http.Dir(staticDir))
	r.Handle("/static/*", http.StripPrefix("/static/", fs))

	uploadFS := http.FileServer(http.Dir(uploadsDir))
	r.Handle("/media-source/*", http.StripPrefix("/media-source/", uploadFS))

	audioFS := http.FileServer(http.Dir(audioDir))
	r.Handle("/media-audio/*", http.StripPrefix("/media-audio/", audioFS))

	previewFS := http.FileServer(http.Dir(previewDir))
	r.Handle("/media-preview/*", http.StripPrefix("/media-preview/", previewFS))

	screenshotFS := http.FileServer(http.Dir(screenshotsDir))
	r.Handle("/media-screenshots/*", http.StripPrefix("/media-screenshots/", screenshotFS))

	// Frontend v0 (old) - /app
	oldFrontendDir := ""
	if len(frontendDirs) > 0 {
		oldFrontendDir = frontendDirs[0]
	}
	if strings.TrimSpace(oldFrontendDir) != "" {
		spaHandler := newSPAHandler(oldFrontendDir, "/app")
		r.Get("/app", spaHandler)
		r.Get("/app/*", spaHandler)
	}

	// Frontend v1 (new) - /app-v1
	newFrontendDir := ""
	if len(frontendDirs) > 1 {
		newFrontendDir = frontendDirs[1]
	}
	if strings.TrimSpace(newFrontendDir) != "" {
		spaHandler := newSPAHandler(newFrontendDir, "/app-v1")
		r.Get("/app-v1", spaHandler)
		r.Get("/app-v1/*", spaHandler)
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
