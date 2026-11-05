package logger

import (
	"context"
	"log/slog"
	"os"
)

// slog wrapper
// in case there is changes in log
type Attr = slog.Attr

var String = slog.String
var Int64 = slog.Int64
var Uint64 = slog.Uint64
var Float64 = slog.Float64
var Bool = slog.Bool
var Time = slog.Time
var Duration = slog.Duration

func Err(err error) Attr {
	errVal := ""
	if err != nil {
		errVal = err.Error()
	}
	return Attr{Key: "error", Value: slog.StringValue(errVal)}
}

var defaultLogger *slog.Logger

func init() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	defaultLogger = slog.New(handler)
}

func Debug(ctx context.Context, msg string, attrs ...Attr) {
	attrs = populateContext(ctx, attrs)
	defaultLogger.LogAttrs(ctx, slog.LevelDebug, msg, attrs...) // kenapa ini tidak bisa
}

func Error(ctx context.Context, msg string, attrs ...Attr) {
	attrs = populateContext(ctx, attrs)
	defaultLogger.LogAttrs(ctx, slog.LevelError, msg, attrs...) // kenapa ini tidak bisa
}
