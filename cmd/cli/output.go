package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// writeOutput publishes one complete file using a temporary file in the same
// directory. Writer, flush, close, and cancellation failures preserve that file's
// previous contents. Each output is replaced independently, not as a batch.
func writeOutput(ctx context.Context, path string, write func(io.Writer) error) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	file, err := os.CreateTemp(directory, ".output-*")
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}
	defer func() {
		file.Close()
		os.Remove(file.Name())
	}()
	if err := write(file); err != nil {
		return fmt.Errorf("write output %s: %w", path, err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("flush output: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close output: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("replace output %s: %w", path, err)
	}
	return nil
}
