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
	"time"

	"github.com/davidimannuel/go-running-route-q-learning/internal/config"
	"github.com/davidimannuel/go-running-route-q-learning/internal/logger"
	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
)

type mapFetcher interface {
	Fetch(context.Context, osm.FetchParam) (osm.OSMData, error)
	CacheKey(osm.FetchParam) string
}

func runDownload(ctx context.Context, args []string, out, logOut io.Writer, fetcher mapFetcher) error {
	flags := flag.NewFlagSet("download", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	workdirFlag := flags.String("workdir", "", "Directory containing config.yaml")
	debug := flags.Bool("debug", false, "Include debug logs on stderr")
	force := flags.Bool("force", false, "Refresh the cached map from Overpass")
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
	ctx = logger.ContextWithLogger(ctx, logger.New(logOut, *debug))
	started := time.Now()
	workdir, err := filepath.Abs(*workdirFlag)
	if err != nil {
		return fmt.Errorf("download: resolve workdir: %w", err)
	}
	cfg, err := config.Load(workdir)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	logger.Info(ctx, "Map preparation started", logger.String("workdir", workdir))
	logger.Debug(ctx, "Configuration loaded",
		logger.String("config_path", filepath.Join(workdir, "config.yaml")),
		logger.String("cache_dir", cfg.CacheDir), logger.String("output_dir", cfg.OutputDir),
		logger.Float64("latitude", cfg.Map.Latitude), logger.Float64("longitude", cfg.Map.Longitude),
		logger.Float64("radius_m", cfg.Map.RadiusM), logger.Int("timeout_sec", cfg.Map.TimeoutSec))
	cache, err := osm.NewCache(filepath.Join(workdir, cfg.CacheDir))
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	params := cfg.Map.FetchParam()
	key := fetcher.CacheKey(params)
	var data osm.OSMData
	var found bool
	if !*force {
		data, found, err = cache.Get(ctx, key)
		if err != nil && !errors.Is(err, osm.ErrInvalidCache) {
			return fmt.Errorf("download: read cache: %w", err)
		}
		if errors.Is(err, osm.ErrInvalidCache) {
			logger.Warn(ctx, "Ignoring invalid cache; fetching fresh OSM data", logger.Err(err))
		}
	}
	if found {
		logger.Info(ctx, "Loaded OSM data from cache", logger.Int("elements", len(data.Elements)))
	} else {
		logger.Info(ctx, "Fetching OSM data from Overpass", logger.Bool("force", *force))
		data, err = fetcher.Fetch(ctx, params)
		if err != nil {
			return fmt.Errorf("download: fetch map: %w", err)
		}
		if err := cache.Set(ctx, key, data); err != nil {
			return fmt.Errorf("download: save cache: %w", err)
		}
		logger.Info(ctx, "Fetched OSM data from Overpass", logger.Int("elements", len(data.Elements)))
	}
	path := filepath.Join(workdir, cfg.OutputDir, "osm.json")
	if err := writeMap(ctx, path, data); err != nil {
		return fmt.Errorf("download: %w", err)
	}
	logger.Info(ctx, "Saved OSM data", logger.String("path", path),
		logger.Int("elements", len(data.Elements)), logger.Duration("duration", time.Since(started)))
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
