package qlearning

import (
	"context"
	"io"
	"math"
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/logger"
)

func TestEpsilonReferenceSchedule(t *testing.T) {
	cfg := DefaultConfig().Training
	cfg.Episodes = 75000
	near(t, EpsilonForEpisode(0, cfg), 1)
	near(t, EpsilonForEpisode(74999, cfg), 0.05)
	horizon := 50000
	cfg.EpsilonDecayEpisodes = &horizon
	near(t, EpsilonForEpisode(49999, cfg), 0.05)
	near(t, EpsilonForEpisode(74999, cfg), 0.05)
	near(t, EpsilonForEpisode(1, cfg), 0.05+0.95*(1-1.0/49999))
}

func TestSeedsAndValidation(t *testing.T) {
	cfg := DefaultConfig().Training
	count := 2
	cfg.Runs = &count
	seeds, err := ResolveSeeds(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(seeds) != 2 || seeds[0] == seeds[1] {
		t.Fatalf("seeds=%v", seeds)
	}
	cfg.Runs = nil
	cfg.Seeds = []uint64{0, 42}
	got, err := ResolveSeeds(cfg)
	if err != nil || !reflect.DeepEqual(got, cfg.Seeds) {
		t.Fatalf("seeds=%v err=%v", got, err)
	}
	for _, mutate := range []func(*TrainingConfig){
		func(c *TrainingConfig) { c.Runs = &count }, func(c *TrainingConfig) { c.Seeds = []uint64{} },
		func(c *TrainingConfig) { c.Seeds = []uint64{0, 0} }, func(c *TrainingConfig) { c.Seeds = []uint64{MaxSeed + 1} },
		func(c *TrainingConfig) { zero := 0; c.EpsilonDecayEpisodes = &zero }, func(c *TrainingConfig) { c.Alpha = math.NaN() },
		func(c *TrainingConfig) { c.EpsilonEnd = 1.1 }, func(c *TrainingConfig) { c.MaxWorkers = 0 },
	} {
		bad := cfg
		mutate(&bad)
		if bad.Validate() == nil {
			t.Fatalf("invalid config accepted: %+v", bad)
		}
	}
}

func TestTrainingSeedReproductionAndGreedyDoesNotUpdateValues(t *testing.T) {
	p := testPreparedGraph(t, [][3]int{{0, 1, 1600}, {1, 2, 1700}, {2, 0, 1700}, {1, 3, 100}, {3, 2, 100}})
	ctx := logger.ContextWithLogger(context.Background(), logger.New(io.Discard, false))
	for _, scenario := range []Scenario{ScenarioA, ScenarioB, ScenarioC} {
		cfg := DefaultConfig()
		cfg.Scenario = scenario
		cfg.Training.Episodes = 25
		var histories [2][]Metrics
		var results [2]TrainingResult
		for run := 0; run < 2; run++ {
			result, err := Train(ctx, p, cfg, 42, func(row Metrics) error { histories[run] = append(histories[run], row); return nil })
			if err != nil {
				t.Fatal(err)
			}
			results[run] = result
		}
		if !reflect.DeepEqual(results[0].Table, results[1].Table) || !reflect.DeepEqual(results[0].EvaluationSteps, results[1].EvaluationSteps) {
			t.Fatal("fixed seed did not reproduce Q-table and route")
		}
		for i := range histories[0] {
			a, b := histories[0][i], histories[1][i]
			// NaN != NaN; compare its absence separately and normalize for equality.
			for _, pair := range [][2]*float64{{&a.MeanAbsTDError, &b.MeanAbsTDError}, {&a.ReturnProgressRatio, &b.ReturnProgressRatio}} {
				if math.IsNaN(*pair[0]) && math.IsNaN(*pair[1]) {
					*pair[0], *pair[1] = 0, 0
				}
			}
			if a != b {
				t.Fatalf("history mismatch at episode %d", i+1)
			}
		}
		snapshot := make(QTable)
		for state, values := range results[0].Table {
			snapshot[state] = make(map[ActionID]float64)
			for id, v := range values {
				snapshot[state][id] = v
			}
		}
		_, steps, err := Evaluate(ctx, p, cfg, 42, results[0].Table)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(steps, results[0].EvaluationSteps) {
			t.Fatal("greedy route did not reproduce")
		}
		if !reflect.DeepEqual(snapshot, results[0].Table) {
			t.Fatal("greedy evaluation changed learned Q-values")
		}
	}
}

func TestGreedyNearTieAndNegativeBootstrap(t *testing.T) {
	p := testPreparedGraph(t, [][3]int{{0, 1, 100}, {0, 2, 100}, {1, 3, 100}})
	e := testEnvironment(t, p, ScenarioA)
	q := make(QTable)
	q[e.State()] = map[ActionID]float64{{NextNode: 1}: 1, {NextNode: 2}: 1.000001}
	seen := map[int]bool{}
	rng := rand.New(rand.NewPCG(0, 0))
	for i := 0; i < 100; i++ {
		id, _ := chooseAction(e, q, rng, 0)
		seen[id.NextNode] = true
	}
	if len(seen) != 2 {
		t.Fatal("near-equal greedy ties were not randomly sampled")
	}
	next := State{Scenario: ScenarioA, CurrentNode: 1, PreviousNode: -1}
	q[next] = map[ActionID]float64{{NextNode: 3}: -3, {NextNode: 0}: 1000}
	id := ActionID{NextNode: 1}
	step, err := TrainStep(e, q, rng, DefaultConfig().Training, 0, &id)
	if err != nil {
		t.Fatal(err)
	}
	near(t, step.MaxNextQ, -3)
}

func TestTrainingCancellationAndRepresentative(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := DefaultConfig()
	cfg.Scenario = ScenarioB
	if _, err := Train(ctx, testPreparedGraph(t, [][3]int{{0, 1, 100}}), cfg, 0, nil); err == nil {
		t.Fatal("cancellation ignored")
	}
	rows := []Metrics{{Seed: 0, AbsoluteDistanceErrorM: 0}, {Seed: 1, IsLoop: true, AbsoluteDistanceErrorM: 10, MeanComfort: 0.5}, {Seed: 2, IsLoop: true, AbsoluteDistanceErrorM: 10, MeanComfort: 0.8}}
	if Representative(rows) != 2 {
		t.Fatal("representative must prefer valid loops, then reference tie ranking")
	}
}

func TestTrainingRequiresResolvedProfile(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Scenario = ScenarioB
	cfg.Training.Profiles = []TrainingProfile{{Name: "short", Episodes: 2}}
	if _, err := Train(context.Background(), nil, cfg, 0, nil); err == nil {
		t.Fatal("unresolved profiles must not silently train with the flat default duration")
	}
	if _, _, err := Evaluate(context.Background(), nil, cfg, 0, make(QTable)); err == nil {
		t.Fatal("evaluation requires a resolved profile configuration")
	}
}
