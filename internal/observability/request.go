package observability

import (
	"context"
	"log/slog"
)

type contextKey string

const requestIDContextKey contextKey = "request_id"

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey, requestID)
}

func RequestIDFromContext(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(requestIDContextKey).(string)
	return value, ok
}

func LoggerFromContext(ctx context.Context, base *slog.Logger) *slog.Logger {
	requestID, _ := RequestIDFromContext(ctx)
	if requestID == "" {
		return base
	}

	return base.With(slog.String("request_id", requestID))
}
