package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, osm.NewOverpassClient()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer, fetcher mapFetcher) error {
	if len(args) == 0 {
		return fmt.Errorf("a command is required; use cli --help")
	}
	switch args[0] {
	case "download":
		return runDownload(ctx, args[1:], out, fetcher)
	case "help", "--help", "-h":
		printUsage(out)
		return nil
	default:
		return fmt.Errorf("unknown command %q; use cli --help", args[0])
	}
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, "Usage: cli download --workdir <directory>")
	fmt.Fprintln(out, "Read <workdir>/config.yaml and save OSM data to <workdir>/<output_dir>/osm.json.")
}
