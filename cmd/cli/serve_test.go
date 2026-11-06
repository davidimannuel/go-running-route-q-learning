package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type previewURLWriter struct {
	urls chan string
}

func (w previewURLWriter) Write(p []byte) (int, error) {
	w.urls <- strings.TrimSpace(string(p))
	return len(p), nil
}

func TestServeCommandAndShutdown(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "map.html"), []byte("<title>Map preview</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data.csv"), []byte("private CSV"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	urls := make(chan string, 1)
	finished := make(chan error, 1)
	fetcher := &fakeFetcher{}
	go func() {
		finished <- run(ctx, []string{"serve", "--dir", root, "--port", "0"}, previewURLWriter{urls}, io.Discard, fetcher)
	}()
	var indexURL string
	select {
	case indexURL = <-urls:
	case err := <-finished:
		t.Fatalf("server did not start: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for preview URL")
	}
	if !strings.HasPrefix(indexURL, "http://127.0.0.1:") || !strings.HasSuffix(indexURL, "/index.html") {
		t.Fatalf("server should use loopback: %s", indexURL)
	}
	client := &http.Client{Timeout: 3 * time.Second}
	for _, tc := range []struct {
		path     string
		status   int
		contains string
	}{
		{"/index.html", http.StatusOK, "/files/map.html"},
		{"/files/map.html", http.StatusOK, "Map preview"},
		{"/files/data.csv", http.StatusNotFound, "404"},
	} {
		response, err := client.Get(strings.TrimSuffix(indexURL, "/index.html") + tc.path)
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil || response.StatusCode != tc.status || !strings.Contains(string(body), tc.contains) {
			t.Fatalf("path=%s, status=%d, body=%s, error=%v", tc.path, response.StatusCode, body, readErr)
		}
	}
	cancel()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("Ctrl+C should stop cleanly: %v", err)
		}
	case <-time.After(7 * time.Second):
		t.Fatal("server did not shut down")
	}
	if fetcher.calls != 0 {
		t.Fatalf("serve triggered %d OSM requests", fetcher.calls)
	}
}

func TestServeValidation(t *testing.T) {
	for _, args := range [][]string{
		{"serve", "--dir", ""}, {"serve", "--dir", "  "},
		{"serve", "--port", "-1"}, {"serve", "--port", "65536"},
		{"serve", "--port", "invalid"}, {"serve", "--unknown"},
		{"serve", "extra"}, {"serve", "--dir", filepath.Join(t.TempDir(), "missing")},
	} {
		if err := run(context.Background(), args, io.Discard, io.Discard, &fakeFetcher{}); err == nil {
			t.Errorf("args=%v should fail", args)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := run(ctx, []string{"serve"}, io.Discard, io.Discard, &fakeFetcher{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation was lost: %v", err)
	}
	var stdout, logs bytes.Buffer
	if err := run(context.Background(), []string{"serve", "--help"}, &stdout, &logs, &fakeFetcher{}); err != nil || logs.Len() != 0 || !strings.Contains(stdout.String(), "serve") {
		t.Fatalf("help output=%s, logs=%s, error=%v", stdout.String(), logs.String(), err)
	}
}

func TestServePortAlreadyInUse(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	err = run(context.Background(), []string{"serve", "--dir", t.TempDir(), "--port", port}, io.Discard, io.Discard, &fakeFetcher{})
	if err == nil || !strings.Contains(err.Error(), "listen on port") {
		t.Fatalf("expected a clear port conflict error: %v", err)
	}
}
