package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/davidimannuel/go-running-route-q-learning/internal/logger"
	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr, osm.NewOverpassClient()); err != nil {
		logger.Error(ctx, "Command failed", logger.Err(err))
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out, logOut io.Writer, fetcher mapFetcher) error {
	if len(args) == 0 {
		return fmt.Errorf("a command is required; use cli --help")
	}
	switch args[0] {
	case "download":
		return runDownload(ctx, args[1:], out, logOut, fetcher)
	case "analyze":
		return runAnalyze(ctx, args[1:], out, logOut)
	case "train":
		return runTrain(ctx, args[1:], out, logOut)
	case "serve":
		return runServe(ctx, args[1:], out, logOut)
	case "help", "--help", "-h":
		printUsage(out)
		return nil
	default:
		return fmt.Errorf("unknown command %q; use cli --help", args[0])
	}
}

func printUsage(out io.Writer) {
	fmt.Fprint(out, `Usage: cli download --workdir <directory> [--force] [--debug]
       cli analyze --workdir <directory> [--debug]
       cli train --workdir <directory> [--snapshot <osm.json>] [--profile <name>] [--debug]
       cli serve [--dir <directory>] [--port <port>] [--debug]

Commands:
  download  Read config.yaml; save osm.json and download.json. --force refreshes the cache.
  analyze   Read the saved snapshot; write nodes.csv, edges.csv, road-analysis.html,
            analysis.json, and analysis.html using the configured scoring parameters.
  train     Run every configured profile, or only --profile. Each profile selects A/B/C.
            Save histories, greedy evaluations, Q-tables, maps, and recorded settings.
            Profile outputs: <output_dir>/training/<profile>/{routes.html,report.html,metadata.json}.
            Each seed also has metadata.json. Profiles share the resolved seed list.
  serve     Browse HTML maps and recorded settings on 127.0.0.1.
            Defaults: --dir . --port 8080. Open the printed URL; Ctrl+C stops the server.
            CSV/JSON/YAML files, hidden entries, and symlinks are not served directly.

Examples (from the repository root):
  go run ./cmd/cli download --workdir workdirs/training
  go run ./cmd/cli analyze --workdir workdirs/training
  go run ./cmd/cli train --workdir workdirs/training
  go run ./cmd/cli train --workdir workdirs/training --profile b-dynamic-150000
  go run ./cmd/cli train --workdir workdirs/training --profile b-dynamic-300000
  go run ./cmd/cli train --workdir workdirs/training --profile b-horizon-100000-total-150000
  go run ./cmd/cli train --workdir workdirs/training --profile b-horizon-100000-total-300000
  go run ./cmd/cli download --workdir workdirs/testing
  go run ./cmd/cli analyze --workdir workdirs/testing
  go run ./cmd/cli train --workdir workdirs/testing
  go run ./cmd/cli train --workdir workdirs/testing --profile b-dynamic-150000
  go run ./cmd/cli train --workdir workdirs/testing --profile b-dynamic-300000
  go run ./cmd/cli train --workdir workdirs/testing --profile b-horizon-100000-total-150000
  go run ./cmd/cli train --workdir workdirs/testing --profile b-horizon-100000-total-300000
  go run ./cmd/cli serve --dir workdirs --port 8080

Both workdirs have 12 A/B/C profiles: 150k/300k, dynamic or horizon 100k.
Training uses seeds 0-4; testing resolves five random seeds once for all selected profiles.
Select a scenario through --profile, not a separate --scenario flag.
With an existing osm.json, run analyze/train directly; no download is needed.
Train always starts fresh Q-tables. A selected rerun replaces only that profile.
Metadata records effective config, reward, actual seeds, snapshot hash, and evaluation.
Logs use stderr/info by default. Train prints summaries to stdout; --debug adds detail.
`)
}
