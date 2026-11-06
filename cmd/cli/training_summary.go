package main

import (
	"fmt"
	"io"
	"time"

	"github.com/davidimannuel/go-running-route-q-learning/internal/analysis"
	"github.com/davidimannuel/go-running-route-q-learning/internal/config"
)

// Keep the default terminal close to the reference notebook: settings, one line
// per completed seed, and one final summary. Episode diagnostics use --debug.
func writeTrainingHeading(out io.Writer, cfg config.Config, seeds []uint64, metadata map[string]any) error {
	name := "single training"
	if profile, ok := metadata["training_profile"].(string); ok {
		name = profile
	}
	training := cfg.QLearning.Training
	horizon := training.Episodes
	mode := "dynamic"
	if training.EpsilonDecayEpisodes != nil {
		horizon = *training.EpsilonDecayEpisodes
		mode = "fixed"
	}
	_, err := fmt.Fprintf(out, "\nScenario %s | %s | %d seeds x %d episodes\nEpsilon %.2f -> %.2f | %s decay: %d episodes | workers: %d | seeds: %v\n",
		cfg.QLearning.Scenario, name, len(seeds), training.Episodes, training.EpsilonStart, training.EpsilonEnd, mode, horizon, min(training.MaxWorkers, len(seeds)), seeds)
	return err
}

func writeTrainingSummary(out io.Writer, trials []analysis.RouteTrial, elapsed time.Duration, target string) error {
	loops := 0
	errorSum := 0.0
	for _, trial := range trials {
		if trial.Metrics.IsLoop {
			loops++
		}
		errorSum += trial.Metrics.AbsoluteDistanceErrorM
	}
	meanError := errorSum / float64(len(trials))
	_, err := fmt.Fprintf(out, "Summary: greedy loops %d/%d | mean distance error %.2f m | elapsed %s\nSaved: %s\n",
		loops, len(trials), meanError, elapsed.Round(time.Millisecond), target)
	return err
}
