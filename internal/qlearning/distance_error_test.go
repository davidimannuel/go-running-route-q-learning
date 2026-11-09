package qlearning

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"testing"
)

func distancePenaltyGraph(t *testing.T, pairs [][3]int, penalty float64) *PreparedGraph {
	t.Helper()
	fixture := testPreparedGraph(t, pairs)
	reward := DefaultRewardConfig()
	reward.DistanceErrorPenaltyPer100M = penalty
	reward.OutwardPenaltyScale, reward.ReturnRewardScale = 0, 0
	p, err := NewPreparedGraph(fixture.graph, 0, reward)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDistanceErrorOnlyAtSuccessfulEndAndNoBootstrap(t *testing.T) {
	for _, tc := range []struct {
		length, bin        int
		penalty, reward, q float64
	}{
		{4750, 13, -12.5, 92.5, 15.575}, {5000, 14, 0, 105, 17.45},
		{5250, 15, -12.5, 92.5, 15.575}, {5050, 14, -2.5, 102.5, 17.075},
	} {
		p := distancePenaltyGraph(t, [][3]int{{0, 1, 2000}, {1, 2, tc.length - 2500}, {2, 0, 500}}, -5)
		e := testEnvironment(t, p, ScenarioC)
		for _, next := range []int{1, 2} {
			step, err := e.Step(ActionID{NextNode: next})
			if err != nil {
				t.Fatal(err)
			}
			near(t, step.Components.DistanceError, 0)
			if step.Done {
				t.Fatal("unfinished route terminated")
			}
		}
		id := ActionID{NextNode: 0}
		q := QTable{e.State(): {id: 2}, {Scenario: ScenarioC, CurrentNode: 0, PreviousNode: 2, ProgressBin: tc.bin}: {{NextNode: 1}: 1e6}}
		step, err := TrainStep(e, q, rand.New(rand.NewPCG(0, 0)), DefaultConfig().Training, 0, &id)
		if err != nil {
			t.Fatal(err)
		}
		// Final edge=5, success=100. 250m error costs 12.5; 50m costs 2.5.
		// Terminal Q=2+.15*(reward-2), without bootstrap or revisit penalty.
		near(t, step.Components.DistanceError, tc.penalty)
		near(t, step.Components.Terminal, 100)
		near(t, step.Components.Direction, 0)
		near(t, step.Components.Revisit, 0)
		near(t, step.Components.Total(), tc.reward)
		near(t, step.Reward, tc.reward)
		near(t, step.MaxNextQ, 0)
		near(t, step.QAfter, tc.q)
		if !step.Done || step.Termination != Success {
			t.Fatal("valid loop no longer succeeds")
		}
		if _, err := advance(e, &id); err == nil {
			t.Fatal("terminal penalty applied twice")
		}
		e.Reset()
		near(t, e.distance, 0)
	}
}

func TestDistanceErrorAddedOnceToAllFailurePaths(t *testing.T) {
	for _, tc := range []struct {
		name                                             string
		pairs                                            [][3]int
		maxSteps                                         int
		path                                             []int
		reason                                           Termination
		errorReward, stepReward, stopReward, totalReward float64
	}{
		{"over distance", [][3]int{{0, 1, 3000}, {1, 2, 2600}}, 220, []int{1, 2}, OverDistance, -30, -54, 0, -54},
		{"step limit", [][3]int{{0, 1, 100}}, 1, []int{1}, StepLimit, -245, -294, 0, -294},
		{"dead end after moving", [][3]int{{0, 1, 100}}, 220, []int{1}, NoAvailableAction, -245, 1, -295, -294},
		{"initial dead end", nil, 220, nil, NoAvailableAction, -250, 0, -300, -300},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := distancePenaltyGraph(t, tc.pairs, -5)
			e := testEnvironment(t, p, ScenarioB)
			e.config.MaxSteps = tc.maxSteps
			q := make(QTable)
			rng := rand.New(rand.NewPCG(0, 0))
			var last StepResult
			var err error
			if len(tc.path) == 0 {
				last, err = TrainStep(e, q, rng, DefaultConfig().Training, 0, nil)
			}
			for _, next := range tc.path {
				id := ActionID{NextNode: next}
				q[e.State()] = map[ActionID]float64{id: 2}
				last, err = TrainStep(e, q, rng, DefaultConfig().Training, 0, &id)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if !last.Done || last.Termination != tc.reason {
				t.Fatalf("termination=%+v", last)
			}
			near(t, last.Components.DistanceError, tc.errorReward)
			near(t, last.RewardStep, tc.stepReward)
			near(t, last.StopPenalty, tc.stopReward)
			near(t, last.Reward, tc.totalReward)
			near(t, last.Components.Total(), tc.totalReward)
			near(t, last.MaxNextQ, 0)
			if last.ActionTaken {
				near(t, last.QAfter, 2+.15*(tc.totalReward-2))
			} else if last.QUpdated || len(q) != 0 {
				t.Fatal("absent action updated Q")
			}
		})
	}
}

func TestDistanceErrorBackwardCompatibilityAndEvaluation(t *testing.T) {
	var old RewardConfig
	if err := json.Unmarshal([]byte(`{"success_bonus":100}`), &old); err != nil {
		t.Fatal(err)
	}
	near(t, old.DistanceErrorPenaltyPer100M, 0)
	near(t, DefaultRewardConfig().DistanceErrorPenaltyPer100M, 0)
	p := distancePenaltyGraph(t, [][3]int{{0, 1, 100}}, -5)
	cfg := DefaultConfig()
	cfg.Scenario = ScenarioB
	cfg.Reward = p.reward
	cfg.Training.Episodes = 2
	cfg.Training.Seeds = []uint64{0}
	result, err := Train(context.Background(), p, cfg, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	evaluation, steps, err := Evaluate(context.Background(), p, cfg, 0, result.Table)
	if err != nil || len(steps) != 1 {
		t.Fatalf("evaluation: %v", err)
	}
	// 100m road=1, dead end=-50, shortfall=4900m costs -245.
	near(t, evaluation.TotalReward, -294)
	near(t, result.Evaluation.TotalReward, -294)
	near(t, steps[0].Components.DistanceError, -245)
	if steps[0].QUpdated {
		t.Fatal("evaluation changed Q")
	}
	cfg.Reward.DistanceErrorPenaltyPer100M = 0
	if _, _, err := Evaluate(context.Background(), p, cfg, 0, result.Table); err == nil {
		t.Fatal("stale prepared reward accepted")
	}
}
