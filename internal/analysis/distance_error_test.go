package analysis

import (
	"bytes"
	"github.com/davidimannuel/go-running-route-q-learning/internal/qlearning"
	"strconv"
	"strings"
	"testing"
)

func TestDistanceErrorComponentExportAndMap(t *testing.T) {
	graph := analysisTestGraph(t)
	edge := &graph.OutgoingEdges(1)[0]
	step := qlearning.StepResult{Transition: qlearning.Transition{
		State:       qlearning.State{Scenario: qlearning.ScenarioB, CurrentNode: edge.From, PreviousNode: -1},
		NextState:   qlearning.State{Scenario: qlearning.ScenarioB, CurrentNode: edge.To, PreviousNode: -1},
		ActionTaken: true, Edge: edge, Done: true, Termination: qlearning.StepLimit,
		Components: qlearning.RewardComponents{Edge: 1, Terminal: -50, DistanceError: -12.5},
	}, RewardStep: -61.5, Reward: -61.5}
	var buf bytes.Buffer
	if err := StepsToCSV(&buf, graph, []qlearning.StepResult{step}); err != nil {
		t.Fatal(err)
	}
	rows := readCSV(t, buf.String())
	columns := map[string]int{}
	for i, name := range rows[0] {
		columns[name] = i
	}
	for name, want := range map[string]float64{"reward_distance_error": -12.5, "reward_terminal": -50, "reward_step": -61.5, "reward": -61.5} {
		index, ok := columns[name]
		if !ok {
			t.Fatalf("missing %s", name)
		}
		got, err := strconv.ParseFloat(rows[1][index], 64)
		if err != nil || got != want {
			t.Fatalf("%s=%v, expected %v", name, got, want)
		}
	}
	buf.Reset()
	trial := RouteTrial{Metrics: qlearning.Metrics{Scenario: qlearning.ScenarioB}, Steps: []qlearning.StepResult{step}}
	if err := RoutesToHTML(&buf, graph, 1, 0, 0, 0, []RouteTrial{trial}, qlearning.DefaultRewardConfig().Road); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"DistanceError":-12.5`) || !strings.Contains(buf.String(), "Distance error penalty") {
		t.Fatal("map lost distance error breakdown")
	}
}
