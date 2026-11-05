package logger

import (
	"context"
	"log/slog"
	"slices"
)

type contextKey int

const (
	keyCorrelationID contextKey = iota
	keyLogger
)

// ContextWithLogger carries a logger through one command and its dependencies.
func ContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, keyLogger, logger)
}

func loggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(keyLogger).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return defaultLogger
}

func ContextWithCorrID(ctx context.Context, corrID string) context.Context {
	return context.WithValue(ctx, keyCorrelationID, corrID)
}

func CorrIDFromContext(ctx context.Context) (string, bool) {
	corrID, ok := ctx.Value(keyCorrelationID).(string)
	return corrID, ok
}

func populateContext(ctx context.Context, attrs []Attr) []Attr {
	if corrID, ok := CorrIDFromContext(ctx); ok && corrID != "" {
		// Preserve caller-owned backing arrays when adding context fields.
		attrs = append(slices.Clip(attrs), String("correlationID", corrID))
	}
	return attrs
}
