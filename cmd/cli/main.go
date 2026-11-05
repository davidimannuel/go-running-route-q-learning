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
	case "help", "--help", "-h":
		printUsage(out)
		return nil
	default:
		return fmt.Errorf("unknown command %q; use cli --help", args[0])
	}
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, "Usage: cli download --workdir <directory> [--force] [--debug]")
	fmt.Fprintln(out, "       cli analyze --workdir <directory> [--debug]")
	fmt.Fprintln(out, "Read <workdir>/config.yaml and save OSM data to <workdir>/<output_dir>/osm.json.")
	fmt.Fprintln(out, "Cache entries are stored in <workdir>/<cache_dir>; --force refreshes the selected entry.")
	fmt.Fprintln(out, "Analyze reads the saved osm.json and always writes nodes.csv and edges.csv in output_dir.")
	fmt.Fprintln(out, "Logs go to stderr at info level by default; --debug includes request and cache details.")
}
