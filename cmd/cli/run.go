package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/davidimannuel/go-running-route-q-learning/internal/config"
	"github.com/davidimannuel/go-running-route-q-learning/internal/logger"
)

// runWorkflow reuses the standalone commands and stops at the first failure.
// It always prepares osm.json for the current map query before analysis/training.
func runWorkflow(ctx context.Context, args []string, out, logOut io.Writer, fetcher mapFetcher) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	workdirFlag := flags.String("workdir", "", "Directory containing config.yaml")
	profile := flags.String("profile", "", "Run only the named training profile; default: all configured profiles")
	force := flags.Bool("force", false, "Refresh the cached map from Overpass")
	debug := flags.Bool("debug", false, "Include debug logs on stderr")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(out)
			return nil
		}
		return fmt.Errorf("run: %w", err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("run: unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if strings.TrimSpace(*workdirFlag) == "" {
		return fmt.Errorf("run: --workdir is required")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	workdir, err := filepath.Abs(*workdirFlag)
	if err != nil {
		return fmt.Errorf("run: resolve workdir: %w", err)
	}
	// Reject invalid training settings before replacing any map or analysis files.
	cfg, err := config.Load(workdir)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	if err := cfg.QLearning.Validate(); err != nil {
		return fmt.Errorf("run: %w; configure qlearning in config.yaml", err)
	}
	if _, err := selectTrainingProfiles(cfg.QLearning.Training, *profile); err != nil {
		return fmt.Errorf("run: %w", err)
	}

	common := []string{"--workdir", workdir}
	if *debug {
		common = append(common, "--debug")
	}
	downloadArgs := append([]string{}, common...)
	if *force {
		downloadArgs = append(downloadArgs, "--force")
	}
	trainArgs := append([]string{}, common...)
	if *profile != "" {
		trainArgs = append(trainArgs, "--profile", *profile)
	}
	ctx = logger.ContextWithLogger(ctx, logger.New(logOut, *debug))
	for i, stage := range []struct {
		name string
		run  func() error
	}{
		{"download", func() error { return runDownload(ctx, downloadArgs, out, logOut, fetcher) }},
		{"analyze", func() error { return runAnalyze(ctx, common, out, logOut) }},
		{"train", func() error { return runTrain(ctx, trainArgs, out, logOut) }},
	} {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("run: before %s: %w", stage.name, err)
		}
		logger.Info(ctx, "Workflow stage started", logger.Int("stage", i+1), logger.String("command", stage.name))
		if err := stage.run(); err != nil {
			return fmt.Errorf("run: %s stage failed: %w", stage.name, err)
		}
	}
	logger.Info(ctx, "Workflow completed", logger.String("workdir", workdir))
	return nil
}
