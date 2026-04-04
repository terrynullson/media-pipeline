package httptransport

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"media-pipeline/internal/transport/http/handlers"
)

func NewRouter(logger *slog.Logger, uploadHandler *handlers.UploadHandler, staticDir string, uploadsDir string, audioDir string, previewDir string, screenshotsDir string) http.Handler {
	r := chi.NewRouter()
	r.Use(RequestIDMiddleware(logger))
	r.Use(AccessLogMiddleware(logger))
	r.Use(RecoverMiddleware(logger))

	r.Get("/", uploadHandler.Index)
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

	return r
}
