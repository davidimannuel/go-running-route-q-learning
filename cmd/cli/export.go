package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/davidimannuel/go-running-route-q-learning/internal/analysis"
	"github.com/davidimannuel/go-running-route-q-learning/internal/config"
	"github.com/davidimannuel/go-running-route-q-learning/internal/logger"
)

type exportSource struct {
	File        string `json:"file"`
	SHA256      string `json:"sha256"`
	CompletedAt string `json:"completed_at"`
}

// runExport refreshes saved HTML in place. Recorded data, not today's training
// parameters, supplies the routes, scores, and settings.
func runExport(ctx context.Context, args []string, out, logOut io.Writer) error {
	flags := flag.NewFlagSet("export", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	workdirFlag := flags.String("workdir", "", "Directory containing config.yaml")
	profile := flags.String("profile", "", "Export only this saved training profile")
	debug := flags.Bool("debug", false, "Include debug logs on stderr")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(out)
			return nil
		}
		return fmt.Errorf("export: %w", err)
	}
	if flags.NArg() != 0 || strings.TrimSpace(*workdirFlag) == "" {
		return fmt.Errorf("export: --workdir is required and positional arguments are not supported")
	}
	ctx = logger.ContextWithLogger(ctx, logger.New(logOut, *debug))
	if err := ctx.Err(); err != nil {
		return err
	}
	workdir, err := filepath.Abs(*workdirFlag)
	if err != nil {
		return err
	}
	cfg, err := config.Load(workdir)
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}
	output := filepath.Join(workdir, cfg.OutputDir)
	root, err := os.OpenRoot(output)
	if err != nil {
		return fmt.Errorf("export: open saved output: %w", err)
	}
	defer root.Close()
	stage, err := os.MkdirTemp(output, ".export-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	var results int
	var sources []exportSource
	logger.Info(ctx, "Exporting saved results", logger.String("map_mode", cfg.Export.Mode()))
	add := func(directory, kind string) error {
		source, err := exportSavedResult(ctx, root, stage, directory, kind, cfg.Export.Mode())
		if err != nil {
			return fmt.Errorf("export %s: %w", directory, err)
		}
		results++
		sources = append(sources, source)
		logger.Debug(ctx, "Exported saved result", logger.String("map", source.File))
		return nil
	}
	// Analysis is optional, and retains its own recorded scoring/date even if old.
	if *profile == "" {
		if _, err := root.Stat("road-analysis.html"); err == nil {
			if err := add(".", "analysis"); err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if *profile != "" {
		if filepath.Base(*profile) != *profile || strings.HasPrefix(*profile, ".") || strings.ContainsAny(*profile, `/\`) {
			return fmt.Errorf("export: invalid profile name")
		}
		if err := add(filepath.Join("training", *profile), "training"); err != nil {
			return err
		}
	} else {
		// Support both current named profiles and legacy single-profile workdirs.
		legacy := false
		if _, err := root.Stat(filepath.Join("training", "routes.html")); err == nil {
			legacy = true
			if err := add("training", "training"); err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		directory, err := root.Open("training")
		if err == nil {
			children, readErr := directory.ReadDir(-1)
			directory.Close()
			if readErr != nil {
				return readErr
			}
			// ReadDir(-1) follows directory order; sort names for a stable catalog.
			names := make([]string, 0, len(children))
			for _, child := range children {
				if !legacy && child.IsDir() && !strings.HasPrefix(child.Name(), ".") {
					names = append(names, child.Name())
				}
			}
			slices.Sort(names)
			for _, name := range names {
				if err := add(filepath.Join("training", name), "training"); err != nil {
					return err
				}
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if results == 0 {
		return fmt.Errorf("export: no saved analysis or training results; run analyze/train first")
	}
	manifest := map[string]any{
		"schema_version": 2, "command": "export", "generated_at": time.Now().UTC().Format(time.RFC3339Nano),
		"map_mode": cfg.Export.Mode(), "workdir": workdir, "sources": sources,
		"training_rerun": false, "research_data_modified": false, "html_updated_in_place": true,
		"external_assets":      []string{"Leaflet 1.9.4 via unpkg.com", "OpenStreetMap raster tiles"},
		"q_values_require_cli": cfg.Export.Mode() == "advanced",
	}
	if err := writeJSON(ctx, filepath.Join(stage, "export.json"), manifest); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	removeSite, err := legacyExportSite(output, workdir)
	if err != nil {
		return fmt.Errorf("export: inspect old site: %w", err)
	}
	if err := publishExport(ctx, output, stage, removeSite); err != nil {
		return fmt.Errorf("export: publish HTML: %w", err)
	}
	if removeSite {
		logger.Info(ctx, "Removed old duplicate HTML export", logger.String("path", filepath.Join(output, "site")))
	}
	logger.Info(ctx, "Updated saved HTML", logger.Int("results", results), logger.String("path", output))
	_, err = fmt.Fprintf(out, "Updated %d saved result(s) in %s (map_mode=%s).\n", results, output, cfg.Export.Mode())
	return err
}

func exportSavedResult(ctx context.Context, root *os.Root, stage, directory, kind, mode string) (exportSource, error) {
	var provenance exportSource
	mapName, metadataName, reportName := "routes.html", "metadata.json", "report.html"
	if kind == "analysis" {
		mapName, metadataName, reportName = "road-analysis.html", "analysis.json", "analysis.html"
	}
	details, err := readExportMetadata(root, filepath.Join(directory, metadataName))
	if err != nil {
		return provenance, err
	}
	if details["status"] != "completed" {
		return provenance, fmt.Errorf("saved result is not completed")
	}
	sourceName := filepath.Join(directory, mapName)
	source, err := root.Open(sourceName)
	if err != nil {
		return provenance, err
	}
	defer source.Close()
	hash := sha256.New()
	reader := io.TeeReader(source, hash)
	// The CLI server supplies All results; Q inspection uses this same map URL.
	options := analysis.HTMLOptions{MapMode: mode}
	if automatic, ok := details["automatic_seeds"].(bool); ok {
		options.SeedSource = analysis.SeedSource(automatic)
	}

	err = writeOutput(ctx, filepath.Join(stage, sourceName), func(w io.Writer) error {
		if kind == "analysis" {
			return analysis.ReexportAnalysisToHTML(w, reader, options)
		}
		summary, err := analysis.ReexportRoutesToHTML(w, reader, options)
		if err != nil {
			return err
		}
		if err := verifyExportIdentity(summary, details); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return provenance, err
	}
	if err := writeOutput(ctx, filepath.Join(stage, directory, reportName), func(w io.Writer) error {
		return analysis.ReportToHTML(w, "Recorded result details", kind, details)
	}); err != nil {
		return provenance, err
	}
	completedAt, _ := details["completed_at"].(string)
	provenance = exportSource{File: filepath.ToSlash(sourceName), SHA256: hex.EncodeToString(hash.Sum(nil)), CompletedAt: completedAt}
	return provenance, nil
}

func readExportMetadata(root *os.Root, name string) (map[string]any, error) {
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 2<<20))
	decoder.UseNumber() // Preserve random uint64 seeds beyond JavaScript's safe integer range.
	var details map[string]any
	if err := decoder.Decode(&details); err != nil {
		return nil, fmt.Errorf("read saved metadata: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("saved metadata must contain one JSON document")
	}
	return details, nil
}

func verifyExportIdentity(summary analysis.SavedRouteSummary, metadata map[string]any) error {
	cfg, _ := metadata["config"].(map[string]any)
	if cfg["scenario"] != summary.Scenario {
		return fmt.Errorf("saved map and metadata scenarios differ")
	}
	seeds, ok := metadata["resolved_seeds"].([]any)
	if !ok || len(seeds) != len(summary.Trials) {
		return fmt.Errorf("saved map and metadata seeds differ")
	}
	for i, seed := range seeds {
		if fmt.Sprint(seed) != summary.Trials[i].ID {
			return fmt.Errorf("saved map and metadata seeds differ")
		}
	}
	return nil
}
