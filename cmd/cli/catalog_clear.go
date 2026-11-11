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

	"github.com/davidimannuel/go-running-route-q-learning/internal/analysis"
	"github.com/davidimannuel/go-running-route-q-learning/internal/logger"
)

// runCatalogClear replaces even an incomplete catalog with a usable empty site.
// It never reads or modifies the training workdir.
func runCatalogClear(ctx context.Context, args []string, out, logOut io.Writer) error {
	flags := flag.NewFlagSet("catalog clear", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	directory := flags.String("dir", "web-catalog", "Catalog directory to empty")
	debug := flags.Bool("debug", false, "Include debug logs on stderr")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(out)
			return nil
		}
		return fmt.Errorf("catalog clear: %w", err)
	}
	if flags.NArg() != 0 || strings.TrimSpace(*directory) == "" {
		return fmt.Errorf("catalog clear: provide a non-empty --dir without positional arguments")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ctx = logger.ContextWithLogger(ctx, logger.New(logOut, *debug))
	target, err := filepath.Abs(*directory)
	if err != nil {
		return err
	}
	if err := validateCatalogDirectory(target); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	unlock, err := lockCatalog(target)
	if err != nil {
		return err
	}
	defer unlock()
	// Recheck after acquiring the same lock used by catalog add.
	if err := validateCatalogDirectory(target); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(target), ".web-catalog-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	empty := catalogManifest{SchemaVersion: 1, MapMode: "standard", Routes: []analysis.CatalogEntry{}}
	if err := writeJSON(ctx, filepath.Join(stage, "catalog.json"), empty); err != nil {
		return err
	}
	if err := writeOutput(ctx, filepath.Join(stage, "index.html"), func(w io.Writer) error { return analysis.CatalogEntriesToHTML(w, empty.Routes) }); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, ".nojekyll"), nil, 0644); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(stage, "routes"), 0755); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := publishTraining(stage, target); err != nil {
		return fmt.Errorf("catalog clear: publish empty catalog: %w", err)
	}
	logger.Info(ctx, "Catalog cleared", logger.String("directory", target))
	_, err = fmt.Fprintf(out, "Catalog cleared: %s\nAll catalog routes removed; download and training results retained.\n", target)
	return err
}

// Share one exclusive lock so add, rm and clear cannot publish over each other.
func lockCatalog(target string) (func(), error) {
	name := filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".lock")
	file, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("catalog: another catalog command may be running (lock %s): %w", name, err)
	}
	file.Close()
	return func() { _ = os.Remove(name) }, nil
}

// Accept missing/broken catalog files, but refuse unrelated data and symlinks.
// No manifest is required: recovering from its accidental deletion is supported.
func validateCatalogDirectory(target string) error {
	info, err := os.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("catalog: target must be a directory, not a file or symlink")
	}
	return filepath.WalkDir(target, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(target, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		parts := strings.Split(relative, string(filepath.Separator))
		allowed := false
		switch len(parts) {
		case 1:
			allowed = (parts[0] == "routes" && entry.IsDir()) || (!entry.IsDir() && (parts[0] == "index.html" || parts[0] == "catalog.json" || parts[0] == ".nojekyll" || parts[0] == ".DS_Store"))
		case 2:
			allowed = parts[0] == "routes" && validCatalogID(parts[1]) && entry.IsDir()
		case 3:
			allowed = parts[0] == "routes" && validCatalogID(parts[1]) && !entry.IsDir() && (parts[2] == "index.html" || parts[2] == "details.html" || parts[2] == "metadata.json" || parts[2] == ".DS_Store")
		}
		if !allowed || entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("catalog: unexpected path %s; choose a catalog directory", relative)
		}
		return nil
	})
}
