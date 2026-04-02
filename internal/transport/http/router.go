package httptransport

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"media-pipeline/internal/transport/http/handlers"
)

func NewRouter(logger *slog.Logger, uploadHandler *handlers.UploadHandler, staticDir string) http.Handler {
	r := chi.NewRouter()
	r.Use(RequestIDMiddleware(logger))
	r.Use(AccessLogMiddleware(logger))
	r.Use(RecoverMiddleware(logger))

	r.Get("/", uploadHandler.Index)
	r.Post("/upload", uploadHandler.Upload)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	fs := http.FileServer(http.Dir(staticDir))
	r.Handle("/static/*", http.StripPrefix("/static/", fs))

	return r
}
