package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/davidimannuel/go-running-route-q-learning/internal/htmlserver"
	"github.com/davidimannuel/go-running-route-q-learning/internal/logger"
)

// runServe previews local HTML independently of download/analyze and config.yaml.
func runServe(ctx context.Context, args []string, out, logOut io.Writer) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	directory := flags.String("dir", ".", "Directory to search recursively for HTML files")
	port := flags.Int("port", 8080, "Local HTTP port; 0 chooses an available port")
	debug := flags.Bool("debug", false, "Include HTTP request logs on stderr")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(out)
			return nil
		}
		return fmt.Errorf("serve: %w", err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("serve: unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if strings.TrimSpace(*directory) == "" {
		return fmt.Errorf("serve: --dir must not be empty")
	}
	if *port < 0 || *port > 65535 {
		return fmt.Errorf("serve: --port must be between 0 and 65535")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	log := logger.New(logOut, *debug)
	ctx = logger.ContextWithLogger(ctx, log)
	root, err := filepath.Abs(*directory)
	if err != nil {
		return fmt.Errorf("serve: resolve directory: %w", err)
	}
	handler, err := htmlserver.New(root)
	if err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	defer handler.Close()
	// Bind only to loopback; this command is a local preview, not a public host.
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(*port)))
	if err != nil {
		return fmt.Errorf("serve: listen on port %d: %w", *port, err)
	}
	defer listener.Close()
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelError),
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	defer server.Close()
	indexURL := "http://" + listener.Addr().String() + "/index.html"
	if _, err := fmt.Fprintln(out, indexURL); err != nil {
		return fmt.Errorf("serve: write preview URL: %w", err)
	}
	logger.Info(ctx, "HTML preview server started", logger.String("directory", root), logger.String("url", indexURL))
	logger.Info(ctx, "Press Ctrl+C to stop the server")
	stopped := make(chan error, 1)
	go func() { stopped <- server.Serve(listener) }()
	select {
	case err := <-stopped:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve: HTTP server: %w", err)
		}
		return nil
	case <-ctx.Done():
		// Shutdown needs a fresh context because the command context is canceled.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("serve: shut down HTTP server: %w", err)
		}
		if err := <-stopped; !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve: HTTP server: %w", err)
		}
		logger.Info(ctx, "HTML preview server stopped")
		return nil
	}
}
