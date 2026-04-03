package httptransport

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"media-pipeline/internal/transport/http/handlers"
)

func NewRouter(logger *slog.Logger, uploadHandler *handlers.UploadHandler, staticDir string, screenshotsDir string) http.Handler {
	r := chi.NewRouter()
	r.Use(RequestIDMiddleware(logger))
	r.Use(AccessLogMiddleware(logger))
	r.Use(RecoverMiddleware(logger))

	r.Get("/", uploadHandler.Index)
	r.Post("/upload", uploadHandler.Upload)
	r.Get("/media/statuses", uploadHandler.MediaStatuses)
	r.Get("/media/{mediaID}/transcript", uploadHandler.Transcript)
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

	screenshotFS := http.FileServer(http.Dir(screenshotsDir))
	r.Handle("/media-screenshots/*", http.StripPrefix("/media-screenshots/", screenshotFS))

	return r
}
