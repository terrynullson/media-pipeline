package httptransport

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"media-pipeline/internal/observability"
)

func RequestIDMiddleware(_ *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := sanitizeRequestID(r.Header.Get("X-Request-ID"))
			if requestID == "" {
				requestID = newRequestID()
			}
			ctx := observability.WithRequestID(r.Context(), requestID)

			w.Header().Set("X-Request-ID", requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RecoverMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					if recorder, ok := w.(*statusRecorder); ok {
						recorder.ensureStatus(http.StatusInternalServerError)
					}
					observability.LoggerFromContext(r.Context(), logger).Error(
						"request panicked",
						slog.Any("panic", rec),
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
						slog.String("stack", string(debug.Stack())),
					)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

func AccessLogMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(recorder, r)

			observability.LoggerFromContext(r.Context(), logger).Info(
				"http request completed",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", recorder.statusCode),
				slog.Int("response_bytes", recorder.bytesWritten),
				slog.Duration("duration", time.Since(startedAt)),
				slog.String("remote_addr", r.RemoteAddr),
			)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
	wroteHeader  bool
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.wroteHeader = true
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *statusRecorder) Write(p []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}

	written, err := r.ResponseWriter.Write(p)
	r.bytesWritten += written
	return written, err
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (r *statusRecorder) ensureStatus(statusCode int) {
	if r.wroteHeader {
		r.statusCode = statusCode
		return
	}
	r.WriteHeader(statusCode)
}

// RequestTimeoutMiddleware wraps each request with http.TimeoutHandler so that
// slow or stalled clients are cut off after timeout. Should NOT be applied to
// the /upload route, which legitimately takes longer for large files.
func RequestTimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, timeout, `{"error":"request timeout"}`)
	}
}

// MediaTokenMiddleware protects routes with a static shared-secret token.
// If token is empty the middleware is a no-op (disabled, backward-compatible).
// Clients must supply the token via:
//   - Header:        Authorization: Bearer <token>
//   - Query param:   ?token=<token>
func MediaTokenMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if token == "" {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var got string
			if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				got = strings.TrimPrefix(auth, "Bearer ")
			} else {
				got = r.URL.Query().Get("token")
			}
			if got != token {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// UploadRateLimitMiddleware limits POST /upload requests to limit per minute per
// IP address using a sliding window. If limit is 0 or negative the middleware
// is disabled (no-op). Exceeding the limit results in 429 Too Many Requests.
func UploadRateLimitMiddleware(limit int64) func(http.Handler) http.Handler {
	if limit <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	var mu sync.Mutex
	// ipTimestamps maps remote IP → sorted slice of request timestamps within the current window.
	ipTimestamps := make(map[string][]time.Time)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}

			now := time.Now()
			windowStart := now.Add(-time.Minute)

			mu.Lock()
			timestamps := ipTimestamps[ip]

			// Evict timestamps outside the sliding window.
			valid := timestamps[:0]
			for _, ts := range timestamps {
				if ts.After(windowStart) {
					valid = append(valid, ts)
				}
			}

			if int64(len(valid)) >= limit {
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"слишком много запросов"}`))
				return
			}

			ipTimestamps[ip] = append(valid, now)
			mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

func sanitizeRequestID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) > 200 {
		return ""
	}

	for _, ch := range value {
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case ch >= '0' && ch <= '9':
		case ch == '-', ch == '_', ch == '.':
		default:
			return ""
		}
	}

	return value
}

func newRequestID() string {
	var randomBytes [8]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	}

	return fmt.Sprintf("%d-%s", time.Now().UTC().UnixNano(), hex.EncodeToString(randomBytes[:]))
}

// LimitRequestBody wraps the request body with http.MaxBytesReader so that
// reads beyond maxBytes fail immediately instead of buffering arbitrary data
// into memory. When the limit is exceeded, json.Decoder returns an error and
// http.MaxBytesReader marks the ResponseWriter so the server returns 413.
//
// Use this on every JSON API endpoint that accepts a request body
// (POST / PUT / PATCH). File-upload endpoints manage their own limit via
// http.MaxBytesReader directly on the multipart reader.
func LimitRequestBody(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
