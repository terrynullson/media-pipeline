package httptransport

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestRequestTimeoutMiddleware_TimeoutTriggered(t *testing.T) {
	t.Parallel()

	slowHandler := RequestTimeoutMiddleware(50 * time.Millisecond)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	slowHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d (timeout)", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestRequestTimeoutMiddleware_FastHandlerNotAffected(t *testing.T) {
	t.Parallel()

	handler := RequestTimeoutMiddleware(5 * time.Second)(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestUploadRateLimitMiddleware_DisabledWhenZero(t *testing.T) {
	t.Parallel()

	handler := UploadRateLimitMiddleware(0)(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	for i := range 20 {
		req := httptest.NewRequest(http.MethodPost, "/upload", nil)
		req.RemoteAddr = "1.2.3.4:5678"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200 (rate limit disabled)", i+1, rec.Code)
		}
	}
}

func TestUploadRateLimitMiddleware_BlocksAfterLimitExceeded(t *testing.T) {
	t.Parallel()

	const limit = 3
	handler := UploadRateLimitMiddleware(limit)(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	for i := range limit {
		req := httptest.NewRequest(http.MethodPost, "/upload", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200 (within limit)", i+1, rec.Code)
		}
	}

	// The limit+1-th request must be rejected.
	req := httptest.NewRequest(http.MethodPost, "/upload", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("request %d: status = %d, want 429", limit+1, rec.Code)
	}
}

func TestUploadRateLimitMiddleware_DifferentIPsAreIndependent(t *testing.T) {
	t.Parallel()

	const limit = 2
	handler := UploadRateLimitMiddleware(limit)(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	// Exhaust limit for IP A.
	for range limit {
		req := httptest.NewRequest(http.MethodPost, "/upload", nil)
		req.RemoteAddr = "192.168.1.1:1111"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// IP B is unaffected.
	req := httptest.NewRequest(http.MethodPost, "/upload", nil)
	req.RemoteAddr = "192.168.1.2:2222"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for unrelated IP", rec.Code)
	}
}

func TestLimitRequestBody_AllowsBodyWithinLimit(t *testing.T) {
	t.Parallel()

	handler := LimitRequestBody(16)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(`{"ok":true}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for body within limit", rec.Code)
	}
}

func TestLimitRequestBody_RejectsOversizedBody(t *testing.T) {
	t.Parallel()

	const limit = 8

	// Handler that reads the full body and reports success; should not be reached.
	handler := LimitRequestBody(limit)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			// MaxBytesReader returned an error — the request is too large.
			http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	body := strings.NewReader(`{"data":"this is longer than 8 bytes"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/test", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413 for oversized body", rec.Code)
	}
}
