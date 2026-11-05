package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// Attr is a structured log field.
type Attr = slog.Attr

var String = slog.String
var Int = slog.Int
var Int64 = slog.Int64
var Uint64 = slog.Uint64
var Float64 = slog.Float64
var Bool = slog.Bool
var Time = slog.Time
var Duration = slog.Duration

func Err(err error) Attr {
	value := ""
	if err != nil {
		value = err.Error()
	}
	return slog.String("error", value)
}

// New writes JSON logs at info level or above unless debug is enabled.
func New(output io.Writer, debug bool) *slog.Logger {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{Level: level}))
}

var defaultLogger = New(os.Stderr, false)

func Debug(ctx context.Context, msg string, attrs ...Attr) {
	log(ctx, slog.LevelDebug, msg, attrs)
}

func Info(ctx context.Context, msg string, attrs ...Attr) {
	log(ctx, slog.LevelInfo, msg, attrs)
}

func Warn(ctx context.Context, msg string, attrs ...Attr) {
	log(ctx, slog.LevelWarn, msg, attrs)
}

func Error(ctx context.Context, msg string, attrs ...Attr) {
	log(ctx, slog.LevelError, msg, attrs)
}

func log(ctx context.Context, level slog.Level, msg string, attrs []Attr) {
	logger := loggerFromContext(ctx)
	if logger.Enabled(ctx, level) {
		logger.LogAttrs(ctx, level, msg, populateContext(ctx, attrs)...)
	}
}
