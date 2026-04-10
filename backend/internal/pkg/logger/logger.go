package logger

import (
	"context"
	"log/slog"
	"os"
)

type contextKey string

const (
	// RequestIDKey is the context key for request ID.
	RequestIDKey contextKey = "request_id"
	// TraceIDKey is the context key for trace ID.
	TraceIDKey contextKey = "trace_id"
)

// Setup initializes the default slog logger with JSON handler.
func Setup() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(handler))
}

// WithContext returns a logger with request_id and trace_id extracted from context.
func WithContext(ctx context.Context) *slog.Logger {
	attrs := []slog.Attr{}

	if reqID, ok := ctx.Value(RequestIDKey).(string); ok && reqID != "" {
		attrs = append(attrs, slog.String("request_id", reqID))
	}

	if traceID, ok := ctx.Value(TraceIDKey).(string); ok && traceID != "" {
		attrs = append(attrs, slog.String("trace_id", traceID))
	}

	if len(attrs) == 0 {
		return slog.Default()
	}

	args := make([]any, len(attrs))
	for i, a := range attrs {
		args[i] = a
	}
	return slog.Default().With(args...)
}
