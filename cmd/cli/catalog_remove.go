package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/davidimannuel/go-running-route-q-learning/internal/analysis"
	"github.com/davidimannuel/go-running-route-q-learning/internal/logger"
)

// runCatalogRemove removes one selected route and rebuilds its catalog index.
// Workdir results and all other catalog routes stay unchanged.
func runCatalogRemove(ctx context.Context, args []string, out, logOut io.Writer) error {
	flags := flag.NewFlagSet("catalog rm", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	directory := flags.String("dir", "web-catalog", "Catalog directory")
	debug := flags.Bool("debug", false, "Include debug logs on stderr")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(out)
			return nil
		}
		return fmt.Errorf("catalog rm: %w", err)
	}
	if flags.NArg() != 1 || !validCatalogID(flags.Arg(0)) || strings.TrimSpace(*directory) == "" {
		return fmt.Errorf("catalog rm: provide one route ID; usage: catalog rm [--dir web-catalog] [--debug] <id>")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ctx = logger.ContextWithLogger(ctx, logger.New(logOut, *debug))
	id := flags.Arg(0)
	target, err := filepath.Abs(*directory)
	if err != nil {
		return err
	}
	if err := validateCatalogDirectory(target); err != nil {
		return err
	}
	unlock, err := lockCatalog(target)
	if err != nil {
		return err
	}
	defer unlock()
	if err := validateCatalogDirectory(target); err != nil {
		return err
	}
	manifest, exists, err := loadCatalog(target)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("catalog rm: catalog does not exist: %s", target)
	}
	index := slices.IndexFunc(manifest.Routes, func(route analysis.CatalogEntry) bool { return route.ID == id })
	if index < 0 {
		return fmt.Errorf("catalog rm: route %q is not in the catalog", id)
	}
	stage, err := os.MkdirTemp(filepath.Dir(target), ".web-catalog-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := os.CopyFS(stage, os.DirFS(target)); err != nil {
		return fmt.Errorf("catalog rm: stage catalog: %w", err)
	}
	if err := os.RemoveAll(filepath.Join(stage, "routes", id)); err != nil {
		return err
	}
	manifest.Routes = slices.Delete(manifest.Routes, index, index+1)
	if err := writeJSON(ctx, filepath.Join(stage, "catalog.json"), manifest); err != nil {
		return err
	}
	if err := writeOutput(ctx, filepath.Join(stage, "index.html"), func(w io.Writer) error { return analysis.CatalogEntriesToHTML(w, manifest.Routes) }); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := publishTraining(stage, target); err != nil {
		return fmt.Errorf("catalog rm: publish catalog: %w", err)
	}
	logger.Info(ctx, "Removed route from catalog", logger.String("id", id), logger.String("directory", target))
	_, err = fmt.Fprintf(out, "Removed catalog route: %s\nCatalog: %s\n", id, filepath.Join(target, "index.html"))
	return err
}
