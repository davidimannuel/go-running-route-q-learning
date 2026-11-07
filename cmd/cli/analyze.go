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

	"github.com/davidimannuel/go-running-route-q-learning/internal/analysis"
	"github.com/davidimannuel/go-running-route-q-learning/internal/config"
	"github.com/davidimannuel/go-running-route-q-learning/internal/logger"
)

// runAnalyze builds the saved snapshot into a raw graph and always exports both
// node/edge CSV tables and a scored interactive map. Downloading is separate.
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
	graph, hash, err := loadTrainingGraph(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("analyze: snapshot missing; run download first: %w", err)
		}
		return fmt.Errorf("analyze: %w", err)
	}
	if err := ctx.Err(); err != nil {
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
		{filename: "nodes.csv", write: func(w io.Writer) error {
			return analysis.NodeToCSV(w, graph)
		}},
		{filename: "edges.csv", write: func(w io.Writer) error {
			return analysis.EdgeToCSV(w, graph, cfg.QLearning.Reward.Road)
		}},
		{filename: "road-analysis.html", write: func(w io.Writer) error {
			return analysis.MapToHTML(w, graph, cfg.Map.Latitude, cfg.Map.Longitude, cfg.QLearning.Reward.Road)
		}},
	} {
		path := filepath.Join(outputDir, export.filename)
		if err := writeOutput(ctx, path, export.write); err != nil {
			return fmt.Errorf("analyze: export %s: %w", export.filename, err)
		}
		message := "Saved graph CSV"
		if export.filename == "road-analysis.html" {
			message = "Saved road analysis map"
		}
		logger.Info(ctx, message, logger.String("path", path))
	}
	metadata := map[string]any{"schema_version": 1, "status": "completed", "command": "analyze",
		"started_at": started.UTC().Format(time.RFC3339Nano), "completed_at": time.Now().UTC().Format(time.RFC3339Nano),
		"workdir_config": cfg, "config_path": filepath.Join(workdir, config.Filename), "config": cfg.QLearning, "snapshot_path": path, "snapshot_sha256": hash,
		"nodes": len(graph.Nodes), "directed_edges": graph.EdgeCount(), "graph_simplified": false,
		"road_score": cfg.QLearning.Reward.Road, "build": buildMetadata(),
		"artifacts": []string{"nodes.csv", "edges.csv", "road-analysis.html", "analysis.json", "analysis.html"}}
	if err := writeJSON(ctx, filepath.Join(outputDir, "analysis.json"), metadata); err != nil {
		return err
	}
	if err := writeOutput(ctx, filepath.Join(outputDir, "analysis.html"), func(w io.Writer) error {
		return analysis.ReportToHTML(w, "Graph analysis details", "analysis", metadata)
	}); err != nil {
		return err
	}
	logger.Info(ctx, "Analysis completed", logger.Duration("duration", time.Since(started)))
	return nil
}
