package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/davidimannuel/go-running-route-q-learning/internal/config"
	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
)

type mapFetcher interface {
	Fetch(context.Context, osm.FetchParam) (osm.OSMData, error)
}

func runDownload(ctx context.Context, args []string, out io.Writer, fetcher mapFetcher) error {
	flags := flag.NewFlagSet("download", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	workdirFlag := flags.String("workdir", "", "Directory containing config.yaml")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(out)
			return nil
		}
		return fmt.Errorf("download: %w", err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("download: unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if strings.TrimSpace(*workdirFlag) == "" {
		return fmt.Errorf("download: --workdir is required")
	}
	workdir, err := filepath.Abs(*workdirFlag)
	if err != nil {
		return fmt.Errorf("download: resolve workdir: %w", err)
	}
	cfg, err := config.Load(workdir)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	data, err := fetcher.Fetch(ctx, cfg.Map.FetchParam())
	if err != nil {
		return fmt.Errorf("download: fetch map: %w", err)
	}
	path := filepath.Join(workdir, cfg.OutputDir, "osm.json")
	if err := writeMap(ctx, path, data); err != nil {
		return fmt.Errorf("download: %w", err)
	}
	fmt.Fprintf(out, "Saved OSM data to %s (%d elements)\n", path, len(data.Elements))
	return nil
}

// writeMap replaces the previous result only after the new file is complete.
func writeMap(ctx context.Context, path string, data osm.OSMData) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("save map: %w", err)
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	file, err := os.CreateTemp(directory, ".osm-*.json")
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}
	defer func() {
		file.Close()
		os.Remove(file.Name())
	}()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("encode map: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("flush map: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close map: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("save map: %w", err)
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("replace map: %w", err)
	}
	return nil
}
