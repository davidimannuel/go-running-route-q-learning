package logger

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
)

func TestLogLevels(t *testing.T) {
	for _, debug := range []bool{false, true} {
		name := "info"
		want := []string{"INFO", "WARN", "ERROR"}
		if debug {
			name = "debug"
			want = append([]string{"DEBUG"}, want...)
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			ctx := ContextWithLogger(context.Background(), New(&out, debug))
			Debug(ctx, "request details")
			Info(ctx, "download started", Int("attempt", 1))
			Warn(ctx, "retry scheduled")
			Error(ctx, "download failed", Err(errors.New("connection refused")))
			var levels []string
			scanner := bufio.NewScanner(&out)
			for scanner.Scan() {
				var entry map[string]any
				if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
					t.Fatal(err)
				}
				levels = append(levels, entry["level"].(string))
				if entry["time"] == nil || entry["msg"] == nil {
					t.Fatalf("missing standard fields: %v", entry)
				}
				if entry["level"] == "INFO" && entry["attempt"] != float64(1) {
					t.Fatalf("structured attribute was lost: %v", entry)
				}
				if entry["level"] == "ERROR" && entry["error"] != "connection refused" {
					t.Fatalf("error field was lost: %v", entry)
				}
			}
			if err := scanner.Err(); err != nil {
				t.Fatal(err)
			}
			if len(levels) != len(want) {
				t.Fatalf("levels=%v; want %v", levels, want)
			}
			for i := range want {
				if levels[i] != want[i] {
					t.Fatalf("levels=%v; want %v", levels, want)
				}
			}
		})
	}
}

func TestDefaultLoggerUsesInfoLevel(t *testing.T) {
	ctx := context.Background()
	if defaultLogger.Enabled(ctx, slog.LevelDebug) || !defaultLogger.Enabled(ctx, slog.LevelInfo) {
		t.Fatal("default logger must enable info and suppress debug")
	}
	if loggerFromContext(ctx) != defaultLogger || loggerFromContext(ContextWithLogger(ctx, nil)) != defaultLogger {
		t.Fatal("missing or nil context logger must use the default")
	}
}

func TestCorrelationIDPreservesCallerAttributes(t *testing.T) {
	var out bytes.Buffer
	ctx := ContextWithLogger(context.Background(), New(&out, false))
	ctx = ContextWithCorrID(ctx, "download-17")
	attrs := make([]Attr, 1, 3)
	attrs[0] = String("source", "cache")
	backing := attrs[:cap(attrs)]
	backing[1] = String("untouched", "value")
	Info(ctx, "map loaded", attrs...)
	var entry map[string]any
	if err := json.Unmarshal(out.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["correlationID"] != "download-17" || entry["source"] != "cache" {
		t.Fatalf("context or caller field was lost: %v", entry)
	}
	if backing[1].Key != "untouched" || backing[1].Value.String() != "value" {
		t.Fatal("logging modified the caller's backing array")
	}
	if Err(nil).Value.String() != "" {
		t.Fatal("nil errors must produce an empty error field")
	}
}
