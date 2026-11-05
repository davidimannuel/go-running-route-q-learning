package main

import (
	"context"
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

// runAnalyze builds the saved snapshot into a raw graph and always exports both
// node and edge CSV tables for notebook analysis. Downloading is a separate command.
func runAnalyze(ctx context.Context, args []string, out, logOut io.Writer) error {
	flags := flag.NewFlagSet("analyze", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	workdirFlag := flags.String("workdir", "", "Directory containing config.yaml")
	debug := flags.Bool("debug", false, "Include debug logs on stderr")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(out)
			return nil
		}
		return fmt.Errorf("analyze: %w", err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("analyze: unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if strings.TrimSpace(*workdirFlag) == "" {
		return fmt.Errorf("analyze: --workdir is required")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("analyze: %w", err)
	}
	ctx = logger.ContextWithLogger(ctx, logger.New(logOut, *debug))
	started := time.Now()
	workdir, err := filepath.Abs(*workdirFlag)
	if err != nil {
		return fmt.Errorf("analyze: resolve workdir: %w", err)
	}
	cfg, err := config.Load(workdir)
	if err != nil {
		return fmt.Errorf("analyze: %w", err)
	}
	outputDir := filepath.Join(workdir, cfg.OutputDir)
	path := filepath.Join(outputDir, "osm.json")
	logger.Info(ctx, "Analyzing OSM snapshot", logger.String("path", path))
	logger.Debug(ctx, "Analysis configuration loaded", logger.String("workdir", workdir), logger.String("output_dir", outputDir))
	data, err := osm.Load(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("analyze: snapshot missing; run download first: %w", err)
		}
		return fmt.Errorf("analyze: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("analyze: %w", err)
	}
	graph := osm.NewGraph()
	if err := graph.LoadOSM(data); err != nil {
		return fmt.Errorf("analyze: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("analyze: %w", err)
	}
	logger.Info(ctx, "Graph built", logger.Int("nodes", len(graph.Nodes)), logger.Int("directed_edges", graph.EdgeCount()))
	for _, export := range []struct {
		filename string
		write    func(io.Writer) error
	}{
		{filename: "nodes.csv", write: graph.NodeToCSV},
		{filename: "edges.csv", write: graph.EdgeToCSV},
	} {
		path := filepath.Join(outputDir, export.filename)
		if err := writeOutput(ctx, path, export.write); err != nil {
			return fmt.Errorf("analyze: export %s: %w", export.filename, err)
		}
		logger.Info(ctx, "Saved graph CSV", logger.String("path", path))
	}
	logger.Info(ctx, "Analysis completed", logger.Duration("duration", time.Since(started)))
	return nil
}
