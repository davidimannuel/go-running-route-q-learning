package main

import (
	"bufio"
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
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/davidimannuel/go-running-route-q-learning/internal/analysis"
	"github.com/davidimannuel/go-running-route-q-learning/internal/config"
	"github.com/davidimannuel/go-running-route-q-learning/internal/logger"
)

type catalogManifest struct {
	SchemaVersion int                     `json:"schema_version"`
	MapMode       string                  `json:"map_mode"`
	Routes        []analysis.CatalogEntry `json:"routes"`
}

func runCatalog(ctx context.Context, args []string, input io.Reader, out, logOut io.Writer) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printUsage(out)
		return nil
	}
	if args[0] == "rm" {
		return runCatalogRemove(ctx, args[1:], out, logOut)
	}
	if args[0] == "clear" {
		return runCatalogClear(ctx, args[1:], out, logOut)
	}
	if args[0] != "add" {
		return fmt.Errorf("catalog: unknown command %q; use catalog add, catalog rm, or catalog clear", args[0])
	}
	flags := flag.NewFlagSet("catalog add", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	workdirFlag := flags.String("workdir", "", "Workdir containing the saved training result")
	profile := flags.String("profile", "", "Saved training profile")
	seedText := flags.String("seed", "", "Actual seed from the saved result")
	directory := flags.String("dir", "web-catalog", "Catalog directory; defaults to web-catalog in the current directory")
	debug := flags.Bool("debug", false, "Include debug logs on stderr")
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(out)
			return nil
		}
		return fmt.Errorf("catalog add: %w", err)
	}
	if flags.NArg() != 0 || strings.TrimSpace(*workdirFlag) == "" || !validCatalogID(*profile) || strings.TrimSpace(*directory) == "" {
		return fmt.Errorf("catalog add: --workdir and a valid --profile are required; positional arguments are not supported")
	}
	seed, err := strconv.ParseUint(*seedText, 10, 64)
	if err != nil || strconv.FormatUint(seed, 10) != *seedText {
		return fmt.Errorf("catalog add: --seed must be the exact saved unsigned integer, including 0")
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
		return err
	}
	// Current config is used only to locate output_dir, never to rebuild results.
	root, err := os.OpenRoot(filepath.Join(workdir, cfg.OutputDir))
	if err != nil {
		return err
	}
	defer root.Close()
	logger.Info(ctx, "Reading saved training result", logger.String("profile", *profile), logger.String("seed", *seedText))
	sourceDir := filepath.Join("training", *profile)
	metadata, err := readExportMetadata(root, filepath.Join(sourceDir, "metadata.json"))
	if err != nil {
		return err
	}
	file, err := root.Open(filepath.Join(sourceDir, "routes.html"))
	if err != nil {
		return err
	}
	hash := sha256.New()
	route, readErr := analysis.ReadCatalogRoute(io.TeeReader(file, hash), *seedText)
	file.Close()
	if readErr != nil {
		return readErr
	}
	if err := verifyExportIdentity(route.Summary, metadata); err != nil {
		return fmt.Errorf("catalog: %w", err)
	}
	if err := route.ValidateMetadata(metadata); err != nil {
		return err
	}
	target, err := filepath.Abs(*directory)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	unlock, err := lockCatalog(target)
	if err != nil {
		return err
	}
	defer unlock()
	manifest, exists, err := loadCatalog(target)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	lines := catalogInputLines(ctx, input)
	id, description, err := promptCatalogDetails(ctx, lines, out, manifest)
	if err != nil {
		return err
	}
	entry := analysis.CatalogEntry{
		ID: id, Description: description, Seed: *seedText, Profile: *profile, Scenario: route.Summary.Scenario,
		DistanceM: route.DistanceM(), AddedAt: time.Now().UTC().Format(time.RFC3339Nano),
		SourceHTMLSHA256: hex.EncodeToString(hash.Sum(nil)), MapMode: "standard", FieldValidation: "not_recorded",
	}
	entry.SourceCompleted, _ = metadata["completed_at"].(string)
	stage, err := os.MkdirTemp(filepath.Dir(target), ".web-catalog-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if exists {
		if err := os.CopyFS(stage, os.DirFS(target)); err != nil {
			return fmt.Errorf("catalog: stage previous catalog: %w", err)
		}
	}
	seedSource := "Seed selection source was not recorded."
	if automatic, ok := metadata["automatic_seeds"].(bool); ok {
		seedSource = analysis.SeedSource(automatic)
	}
	metadata["catalog"] = entry // The source JSON file itself is never changed.
	routeDir := filepath.Join(stage, "routes", id)
	for _, artifact := range []struct {
		name  string
		write func(io.Writer) error
	}{
		{"index.html", func(w io.Writer) error { return route.ToHTML(w, entry, seedSource) }},
		{"details.html", func(w io.Writer) error {
			return analysis.ReportToHTMLWithLinks(w, "Route details · "+id, "catalog", metadata, "../../index.html", "index.html")
		}},
	} {
		if err := writeOutput(ctx, filepath.Join(routeDir, artifact.name), artifact.write); err != nil {
			return err
		}
	}
	if err := writeJSON(ctx, filepath.Join(routeDir, "metadata.json"), metadata); err != nil {
		return err
	}
	manifest.Routes = append(manifest.Routes, entry)
	slices.SortFunc(manifest.Routes, func(a, b analysis.CatalogEntry) int { return strings.Compare(a.ID, b.ID) })
	if err := writeJSON(ctx, filepath.Join(stage, "catalog.json"), manifest); err != nil {
		return err
	}
	if err := writeOutput(ctx, filepath.Join(stage, "index.html"), func(w io.Writer) error { return analysis.CatalogEntriesToHTML(w, manifest.Routes) }); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, ".nojekyll"), nil, 0o644); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := publishTraining(stage, target); err != nil {
		return fmt.Errorf("catalog: publish catalog: %w", err)
	}
	logger.Info(ctx, "Added saved route to catalog", logger.String("id", id), logger.String("seed", *seedText), logger.String("map_mode", "standard"))
	_, err = fmt.Fprintf(out, "Added: %s\nCatalog: %s\n", filepath.Join(target, "routes", id, "index.html"), filepath.Join(target, "index.html"))
	return err
}

func validCatalogID(id string) bool {
	if len(id) == 0 || len(id) > 64 || id[0] == '-' || id[len(id)-1] == '-' {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '-' {
			return false
		}
	}
	return true
}

func loadCatalog(directory string) (catalogManifest, bool, error) {
	empty := catalogManifest{SchemaVersion: 1, MapMode: "standard", Routes: []analysis.CatalogEntry{}}
	root, err := os.OpenRoot(directory)
	if errors.Is(err, os.ErrNotExist) {
		return empty, false, nil
	}
	if err != nil {
		return empty, false, err
	}
	defer root.Close()
	if _, err := root.Stat(".git"); err == nil {
		return empty, false, fmt.Errorf("catalog: directory must not be a Git checkout root")
	}
	file, err := root.Open("catalog.json")
	if err != nil {
		return empty, false, fmt.Errorf("catalog: existing directory requires catalog.json (use catalog clear to reset an incomplete catalog): %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() > 4<<20 {
		return empty, false, fmt.Errorf("catalog: invalid or oversized catalog.json")
	}
	decoder := json.NewDecoder(file)
	var manifest catalogManifest
	if err := decoder.Decode(&manifest); err != nil {
		return empty, false, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return empty, false, fmt.Errorf("catalog: catalog.json must contain one document")
	}
	if manifest.SchemaVersion != 1 || manifest.MapMode != "standard" {
		return empty, false, fmt.Errorf("catalog: unsupported schema or map mode")
	}
	ids := make(map[string]bool)
	for _, route := range manifest.Routes {
		if !validCatalogID(route.ID) || ids[route.ID] || route.MapMode != "standard" {
			return empty, false, fmt.Errorf("catalog: invalid or duplicate route ID/mode")
		}
		ids[route.ID] = true
		for _, name := range []string{"index.html", "details.html", "metadata.json"} {
			if _, err := root.Stat(filepath.Join("routes", route.ID, name)); err != nil {
				return empty, false, fmt.Errorf("catalog: incomplete existing route %s: %w", route.ID, err)
			}
		}
	}
	// Unlisted route folders must not be silently overwritten by a new entry.
	if directory, err := root.Open("routes"); err == nil {
		children, readErr := directory.ReadDir(-1)
		directory.Close()
		if readErr != nil {
			return empty, false, readErr
		}
		for _, child := range children {
			if !ids[child.Name()] {
				return empty, false, fmt.Errorf("catalog: unlisted route path %s", child.Name())
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return empty, false, err
	}
	return manifest, true, nil
}

type catalogInput struct {
	text string
	err  error
}

// One scanner permits cancellation while the terminal is waiting for input.
func catalogInputLines(ctx context.Context, input io.Reader) <-chan catalogInput {
	lines := make(chan catalogInput)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(input)
		scanner.Buffer(make([]byte, 1024), 8192)
		for scanner.Scan() {
			select {
			case lines <- catalogInput{text: scanner.Text()}:
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			select {
			case lines <- catalogInput{err: err}:
			case <-ctx.Done():
			}
		}
	}()
	return lines
}

func promptCatalogDetails(ctx context.Context, lines <-chan catalogInput, out io.Writer, manifest catalogManifest) (string, string, error) {
	read := func(label string) (string, error) {
		if _, err := fmt.Fprint(out, label); err != nil {
			return "", err
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case line, ok := <-lines:
			if !ok {
				return "", fmt.Errorf("catalog: input ended; nothing was added")
			}
			return strings.TrimSpace(line.text), line.err
		}
	}
	var id string
	for {
		var err error
		id, err = read("Route ID: ")
		if err != nil {
			return "", "", err
		}
		duplicate := slices.ContainsFunc(manifest.Routes, func(route analysis.CatalogEntry) bool { return route.ID == id })
		if validCatalogID(id) && !duplicate {
			break
		}
		if _, err := fmt.Fprintln(out, "Use an unused ID of 1–64 lowercase letters/digits with optional interior hyphens."); err != nil {
			return "", "", err
		}
	}
	for {
		description, err := read("Route description: ")
		if err != nil {
			return "", "", err
		}
		if description != "" && utf8.RuneCountInString(description) <= 1000 {
			return id, description, nil
		}
		if _, err := fmt.Fprintln(out, "Enter a description of 1–1000 characters."); err != nil {
			return "", "", err
		}
	}
}
