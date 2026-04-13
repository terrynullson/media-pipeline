package httptransport

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"media-pipeline/internal/observability"
)

func TestMediaTokenMiddleware_DisabledWhenEmpty(t *testing.T) {
	t.Parallel()

	handler := MediaTokenMiddleware("")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/media-source/file.mp4", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 when token disabled", rec.Code)
	}
}

func TestMediaTokenMiddleware_RejectsWithoutToken(t *testing.T) {
	t.Parallel()

	handler := MediaTokenMiddleware("secret123")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/media-source/file.mp4", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 when token missing", rec.Code)
	}
}

func TestMediaTokenMiddleware_AcceptsBearerHeader(t *testing.T) {
	t.Parallel()

	handler := MediaTokenMiddleware("secret123")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/media-source/file.mp4", nil)
	req.Header.Set("Authorization", "Bearer secret123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with correct Bearer token", rec.Code)
	}
}

func TestMediaTokenMiddleware_AcceptsQueryParam(t *testing.T) {
	t.Parallel()

	handler := MediaTokenMiddleware("secret123")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/media-source/file.mp4?token=secret123", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with correct query param token", rec.Code)
	}
}

func TestMediaTokenMiddleware_RejectsWrongToken(t *testing.T) {
	t.Parallel()

	handler := MediaTokenMiddleware("secret123")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/media-source/file.mp4", nil)
	req.Header.Set("Authorization", "Bearer wrongtoken")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 with wrong token", rec.Code)
	}
}

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
