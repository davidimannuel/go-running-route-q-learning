package qlearning

import (
	"context"
	"math/rand/v2"
	"testing"
)

func TestConfiguredRewardReachesScoringAccessAndBellman(t *testing.T) {
	p := testPreparedGraph(t, [][3]int{{0, 1, 100}, {1, 2, 100}, {2, 3, 100}})
	p.graph.OutgoingEdges(0)[0].Access = "permissive"
	cfg := DefaultConfig()
	cfg.Scenario = ScenarioB
	cfg.Reward.Road.Highway.Priority = 2
	cfg.Reward.PermissivePenaltyPer100M = -0.3
	cfg.Reward.NoActionPenalty = -7
	var err error
	p, err = NewPreparedGraph(p.graph, 0, cfg.Reward)
	if err != nil {
		t.Fatal(err)
	}
	env, err := NewEnvironment(p, cfg.Scenario, cfg.Environment)
	if err != nil {
		t.Fatal(err)
	}
	table := make(QTable)
	id := ActionID{1, 0}
	step, err := TrainStep(env, table, rand.New(rand.NewPCG(1, 0)), cfg.Training, 0, &id)
	if err != nil {
		t.Fatal(err)
	}
	// Independent: comfort=.55*2+.30*1+.15*1=1.55; r=1.55-.3=1.25;
	// unseen next Q=0, alpha=.15 => updated Q=.1875.
	near(t, step.Components.Edge, 1.55)
	near(t, step.Components.Access, -0.3)
	near(t, step.Reward, 1.25)
	near(t, step.QAfter, 0.1875)
	different := cfg
	different.Reward.SuccessBonus = 200
	if _, err := Train(context.Background(), p, different, 1, nil); err == nil {
		t.Fatal("reward/cache mismatch accepted")
	}
}

func TestConfiguredTerminalAndDeadEndRewards(t *testing.T) {
	p := testPreparedGraph(t, [][3]int{{0, 1, 2500}, {1, 2, 2000}, {2, 0, 500}})
	reward := DefaultRewardConfig()
	reward.SuccessBonus = 37
	reward.NoActionPenalty = -7
	reward.StepLimitPenalty = -9
	reward.OverDistancePenalty = -11
	reward.RevisitPenalty = -3
	reward.OutwardPenaltyScale = 0
	reward.ReturnRewardScale = 0
	p, err := NewPreparedGraph(p.graph, 0, reward)
	if err != nil {
		t.Fatal(err)
	}
	e := testEnvironment(t, p, ScenarioB)
	for _, next := range []int{1, 2, 0} {
		step, err := e.Step(ActionID{next, 0})
		if err != nil {
			t.Fatal(err)
		}
		if next == 0 {
			near(t, step.Components.Terminal, 37)
		}
	}
	p = testPreparedGraph(t, [][3]int{{0, 1, 100}})
	p, err = NewPreparedGraph(p.graph, 0, reward)
	if err != nil {
		t.Fatal(err)
	}
	e = testEnvironment(t, p, ScenarioB)
	id := ActionID{1, 0}
	step, err := advance(e, &id)
	if err != nil {
		t.Fatal(err)
	}
	near(t, step.StopPenalty, -7)
	near(t, step.Reward, -6)
	e = testEnvironment(t, p, ScenarioB)
	e.config.MaxSteps = 1
	terminal, err := e.Step(id)
	if err != nil {
		t.Fatal(err)
	}
	near(t, terminal.Components.Terminal, -9)
	e = testEnvironment(t, p, ScenarioB)
	e.config.TargetDistanceM = 50
	terminal, err = e.Step(id)
	if err != nil {
		t.Fatal(err)
	}
	near(t, terminal.Components.Terminal, -11)
}
