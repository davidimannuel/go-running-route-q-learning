package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/davidimannuel/go-running-route-q-learning/internal/analysis"
	"github.com/davidimannuel/go-running-route-q-learning/internal/config"
	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
	"github.com/davidimannuel/go-running-route-q-learning/internal/qlearning"
)

type trainingEvent struct {
	index  int
	row    qlearning.Metrics
	result *qlearning.TrainingResult
	err    error
}

// Workers share immutable prepared graph data only. The parent consumes a
// bounded channel, writes all files, and stores final evaluations in seed order.
// No shared Q-table/RNG and no millions-of-episodes history retained in memory.
func trainBatch(parent context.Context, out io.Writer, p *qlearning.PreparedGraph, cfg qlearning.Config, seeds []uint64, stage string) ([]analysis.RouteTrial, error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	files := make([]*os.File, len(seeds))
	writers := make([]*analysis.MetricsCSV, len(seeds))
	defer func() {
		for _, file := range files {
			if file != nil {
				file.Close()
			}
		}
	}()
	for i := range seeds {
		file, err := os.Create(filepath.Join(stage, fmt.Sprintf(".history-%d.csv", i)))
		if err != nil {
			return nil, err
		}
		files[i] = file
		writers[i], err = analysis.NewMetricsCSV(file)
		if err != nil {
			return nil, err
		}
	}
	jobs := make(chan int)
	events := make(chan trainingEvent, 128)
	var workers sync.WaitGroup
	for worker := 0; worker < min(cfg.Training.MaxWorkers, len(seeds)); worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				if ctx.Err() != nil {
					return
				}
				result, err := qlearning.Train(ctx, p, cfg, seeds[index], func(row qlearning.Metrics) error {
					select {
					case events <- trainingEvent{index: index, row: row}:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})
				select {
				case events <- trainingEvent{index: index, result: &result, err: err}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for index := range seeds {
			select {
			case jobs <- index:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() { workers.Wait(); close(events) }()
	trials := make([]analysis.RouteTrial, len(seeds))
	completed := 0
	var firstErr error
	for event := range events {
		if firstErr != nil {
			continue
		}
		if event.err != nil {
			firstErr = event.err
			cancel()
			continue
		}
		if event.result == nil {
			if err := writers[event.index].Write(event.row); err != nil {
				firstErr = err
				cancel()
			}
			continue
		}
		result := event.result
		trials[event.index] = analysis.RouteTrial{Metrics: result.Evaluation, Steps: result.EvaluationSteps}
		directory := filepath.Join(stage, "seed-"+strconv.FormatUint(seeds[event.index], 10))
		if err := writeOutput(ctx, filepath.Join(directory, "qtable.csv"), func(w io.Writer) error { return analysis.QTableToCSV(w, result.Table) }); err != nil {
			firstErr = err
			cancel()
			continue
		}
		if err := writeOutput(ctx, filepath.Join(directory, "evaluation-steps.csv"), func(w io.Writer) error { return analysis.StepsToCSV(w, p.Graph(), result.EvaluationSteps) }); err != nil {
			firstErr = err
			cancel()
			continue
		}
		completed++ // Large Q-tables can be released after this event is consumed.
		m := result.Evaluation
		if _, err := fmt.Fprintf(out, "[%d/%d] seed=%d: %s, %.3f km, is_loop=%t\n", completed, len(seeds), m.Seed, m.TerminationReason, m.TotalDistanceM/1000, m.IsLoop); err != nil {
			firstErr = err
			cancel()
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	if err := parent.Err(); err != nil {
		return nil, err
	}
	if completed != len(seeds) {
		return nil, fmt.Errorf("training interrupted before all seeds completed")
	}
	for i, writer := range writers {
		if err := writer.Flush(); err != nil {
			return nil, err
		}
		if err := files[i].Close(); err != nil {
			return nil, err
		}
		files[i] = nil
	}
	return trials, nil
}

func finishTrainingFiles(ctx context.Context, p *qlearning.PreparedGraph, cfg config.Config, trials []analysis.RouteTrial, seeds []uint64, stage string) error {
	// Concatenation uses input seed order regardless of worker completion order.
	// Read only one CSV header at a time; the remaining rows stream through io.Copy.
	if err := writeOutput(ctx, filepath.Join(stage, "history.csv"), func(w io.Writer) error {
		for i := range seeds {
			if err := appendHistory(w, filepath.Join(stage, fmt.Sprintf(".history-%d.csv", i)), i > 0); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	for i := range seeds {
		if err := os.Remove(filepath.Join(stage, fmt.Sprintf(".history-%d.csv", i))); err != nil {
			return err
		}
	}
	if err := writeOutput(ctx, filepath.Join(stage, "evaluation.csv"), func(w io.Writer) error {
		writer, err := analysis.NewMetricsCSV(w)
		if err != nil {
			return err
		}
		for _, trial := range trials {
			if err := writer.Write(trial.Metrics); err != nil {
				return err
			}
		}
		return writer.Flush()
	}); err != nil {
		return err
	}
	node := p.Graph().Nodes[p.StartNode()]
	snapM, err := osm.GreatCircleDistanceM(cfg.Map.Latitude, cfg.Map.Longitude, node.Lat, node.Lon)
	if err != nil {
		return err
	}
	return writeOutput(ctx, filepath.Join(stage, "routes.html"), func(w io.Writer) error {
		return analysis.RoutesToHTMLWithOptions(w, p.Graph(), p.StartNode(), cfg.Map.Latitude, cfg.Map.Longitude, snapM, trials, cfg.QLearning.Reward.Road, analysis.HTMLOptions{MapMode: cfg.Export.Mode(), SeedSource: analysis.SeedSource(cfg.QLearning.Training.Seeds == nil)})
	})
}

func appendHistory(w io.Writer, path string, skipHeader bool) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	if skipHeader {
		if _, err := reader.ReadString('\n'); err != nil {
			return err
		}
	}
	_, err = io.Copy(w, reader)
	return err
}
