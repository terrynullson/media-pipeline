package httptransport

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"media-pipeline/internal/observability"
)

func TestRequestIDMiddleware_ReusesIncomingHeader(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := RequestIDMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID, ok := observability.RequestIDFromContext(r.Context())
		if !ok {
			t.Fatal("request id missing from context")
		}
		if requestID != "incoming-id-123" {
			t.Fatalf("request id = %q, want %q", requestID, "incoming-id-123")
		}

		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("X-Request-ID", "incoming-id-123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != "incoming-id-123" {
		t.Fatalf("response X-Request-ID = %q, want %q", got, "incoming-id-123")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}
