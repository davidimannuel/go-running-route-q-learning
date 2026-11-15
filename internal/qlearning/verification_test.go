package qlearning

import (
	"cmp"
	"encoding/csv"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// Core execution: agent.go: TrainStep -> advance -> environment.go: Environment.Step.
// This test supplies actions and expected results; production code computes transitions and Q updates.
func TestVerificationEpisodes(t *testing.T) {
	// Independent expected values. Each scenario starts with an empty Q-table.
	type expectedStep struct {
		next, bin                                   int
		distance, reward, qBefore, maxNextQ, qAfter float64
		legal                                       string
	}
	type expectedEpisode struct {
		steps                   []expectedStep
		totalReward, finalError float64
		termination             Termination
		storedPairs             int
	}
	cases := []struct {
		scenario Scenario
		maxSteps int
		episodes []expectedEpisode
	}{
		{ScenarioA, 500, []expectedEpisode{
			{[]expectedStep{
				{2, 0, 973.4, 9.734, 0, 0, 1.4601, "(2,0) (5,0)"},
				{3, 0, 2101, 11.276, 0, 0, 1.6914, "(3,0) (3,1)"},
				{6, 0, 2318.3, -197.01435, 0, 0, -29.5521525, "(4,0) (6,0)"},
			}, -176.00435, -134.085, NoAvailableAction, 3},
			{[]expectedStep{
				{2, 0, 973.4, 9.734, 1.4601, 1.6914, 2.9422095, "(2,0) (5,0)"},
				{3, 0, 2101, 11.276, 1.6914, 0, 3.12909, "(3,0) (3,1)"},
				{4, 0, 3047.8, 8.5212, 0, 0, 1.27818, "(4,0) (6,0)"},
				{5, 0, 4131, 6.1528, 0, 0, .92292, "(5,0) (3,0)"},
				{1, 0, 4995.7, 88.327725, 0, 0, 13.24915875, "(4,0) (1,0)"},
			}, 124.011725, -.215, Success, 6},
		}},
		{ScenarioB, 500, []expectedEpisode{
			{[]expectedStep{
				{2, 4, 973.4, 9.734, 0, 0, 1.4601, "(2,0) (5,0)"},
				{3, 8, 2101, 11.276, 0, 0, 1.6914, "(3,0) (3,1)"},
				{4, 12, 3047.8, 8.5212, 0, 0, 1.27818, "(4,0) (6,0)"},
				{5, 17, 4131, 6.1528, 0, 0, .92292, "(5,0) (3,0)"},
				{4, 21, 5214.2, -16.6944, 0, 0, -2.50416, "(4,0) (1,0)"},
				{3, 24, 6161, -109.5288, 0, 0, -16.42932, "(3,0) (5,0)"},
			}, -90.5392, -58.05, OverDistance, 6},
			{[]expectedStep{
				{2, 4, 973.4, 9.734, 1.4601, 1.6914, 2.9422095, "(2,0) (5,0)"},
				{3, 8, 2101, 11.276, 1.6914, 1.27818, 3.31123065, "(3,0) (3,1)"},
				{4, 12, 3047.8, 8.5212, 1.27818, .92292, 2.4961491, "(4,0) (6,0)"},
				{5, 17, 4131, 6.1528, .92292, 0, 1.707402, "(5,0) (3,0)"},
				{1, 20, 4995.7, 88.327725, 0, 0, 13.24915875, "(4,0) (1,0)"},
			}, 124.011725, -.215, Success, 7},
		}},
		// Five steps deliberately replaces the default 500 ONLY in this test.
		// The second episode succeeds on step 5, checking success priority too.
		{ScenarioC, 5, []expectedEpisode{
			{[]expectedStep{
				{2, 4, 973.4, 9.734, 0, 0, 1.4601, "(2,0) (5,0)"},
				{3, 8, 2101, 11.276, 0, 0, 1.6914, "(3,0) (3,1)"},
				{4, 12, 3047.8, 8.5212, 0, 0, 1.27818, "(4,0) (6,0)"},
				{3, 16, 3994.6, -3.8788, 0, 0, -.58182, "(5,0) (3,0)"},
				{2, 21, 5122.2, -54.834, 0, 0, -8.2251, "(6,0) (2,0) (2,1) (4,0)"},
			}, -29.1816, -6.11, StepLimit, 5},
			{[]expectedStep{
				{2, 4, 973.4, 9.734, 1.4601, 1.6914, 2.9422095, "(2,0) (5,0)"},
				{3, 8, 2101, 11.276, 1.6914, 1.27818, 3.31123065, "(3,0) (3,1)"},
				{4, 12, 3047.8, 8.5212, 1.27818, 0, 2.364633, "(4,0) (6,0)"},
				{5, 17, 4131, 6.1528, 0, 0, .92292, "(5,0) (3,0)"},
				{1, 20, 4995.7, 88.327725, 0, 0, 13.24915875, "(4,0) (1,0)"},
			}, 124.011725, -.215, Success, 7},
		}},
	}

	for _, tc := range cases {
		t.Run(string(tc.scenario), func(t *testing.T) {
			graph := buildSyntheticGraph(t)
			cfg := DefaultConfig()
			if tc.scenario == ScenarioC {
				cfg.Environment.MaxSteps = tc.maxSteps
				t.Log("Test-only max_steps=5 (training default remains 500)")
			}
			if cfg.Environment.MaxSteps != tc.maxSteps {
				t.Fatal("unexpected step limit")
			}
			// Core: prepared_graph.go: NewPreparedGraph caches scores and builds access-filtered Actions.
			prepared, err := NewPreparedGraph(graph, 1, cfg.Reward)
			if err != nil {
				t.Fatal(err)
			}
			// Explicit synthetic distances to start, NOT Haversine measurements.
			prepared.distances = map[int]float64{1: 0, 2: 973.4, 3: 1600, 4: 1400, 5: 864.7, 6: 1750, 7: 1100}
			env, err := NewEnvironment(prepared, tc.scenario, cfg.Environment)
			if err != nil {
				t.Fatal(err)
			}
			q := make(QTable)
			// TrainStep requires an RNG, but explicit actions bypass policy sampling.
			rng := rand.New(rand.NewPCG(1, 2))
			t.Logf("Scenario %s; initial Q-table empty; explicit actions; alpha=%.2f; gamma=%.2f; max_steps=%d",
				tc.scenario, cfg.Training.Alpha, cfg.Training.Gamma, cfg.Environment.MaxSteps)
			t.Logf("Synthetic distances to start (m): %v", prepared.distances)
			var failedState State
			var failedAction ActionID
			var failedQ float64
			for episodeIndex, episode := range tc.episodes {
				state := env.Reset() // Reset ONLY the environment; retain this scenario's Q-table.
				if state != (State{Scenario: tc.scenario, CurrentNode: 1, PreviousNode: -1}) || env.distance != 0 || env.steps != 0 || len(env.visits) != 1 || env.visits[1] != 1 {
					t.Fatal("environment reset leaked episode state")
				}
				t.Logf("EPISODE %d", episodeIndex+1)
				totalReward := 0.0
				previous := 1
				var last StepResult
				for stepIndex, want := range episode.steps {
					logVerificationMask(t, env, want.legal)
					state := env.State()
					action := ActionID{NextNode: want.next, Key: 0}
					// Core: agent.go: TrainStep skips chooseAction when &action is supplied.
					// advance applies movement/dead-end rewards; TrainStep performs the Bellman update.
					result, err := TrainStep(env, q, rng, cfg.Training, 0, &action)
					if err != nil {
						t.Fatalf("episode %d step %d: %v", episodeIndex+1, stepIndex+1, err)
					}
					// Core: environment.go: Environment.State constructs A/B/C fields after Step.
					wantState := State{Scenario: tc.scenario, CurrentNode: want.next, ProgressBin: want.bin, PreviousNode: -1}
					if tc.scenario == ScenarioC {
						wantState.PreviousNode = previous
					}
					t.Logf("Step %d | State: %s; action: (%d,%d); next state: %s; distance=%.1f m",
						stepIndex+1, verificationState(state), action.NextNode, action.Key,
						verificationState(result.NextState), result.TotalDistanceM)
					// Core: scoring.go: RoadScoreConfig.ScoreEdge; actual rewards come from result.Components.
					scores := cfg.Reward.Road.ScoreEdge(*result.Edge)
					road := cfg.Reward.Road
					cachedScore := prepared.edges[result.Edge].comfort
					t.Logf("Road score: %.2f*%.2f + %.2f*%.2f + %.2f*%.2f = %.5f",
						road.Highway.Weight, scores.HighwayScore, road.Surface.Weight, scores.SurfaceScore,
						road.Width.Weight, scores.WidthScore, cachedScore)
					t.Logf("Road reward: %.5f * (%.1f/100) = %.8f", cachedScore, result.Edge.LengthM, result.Components.Edge)
					// Show extra reward terms only when they contribute to this step.
					parts := []string{fmt.Sprintf("road=%.8f", result.Components.Edge)}
					for _, component := range []struct {
						name  string
						value float64
					}{
						{"access", result.Components.Access}, {"direction", result.Components.Direction},
						{"revisit", result.Components.Revisit}, {"terminal", result.Components.Terminal},
						{"no_action", result.Components.NoAction}, {"distance_error", result.Components.DistanceError},
					} {
						if component.value != 0 {
							parts = append(parts, fmt.Sprintf("%s=%.8f", component.name, component.value))
						}
					}
					if len(parts) > 1 {
						t.Logf("Reward total: %s = %.8f", strings.Join(parts, " + "), result.Reward)
					}
					t.Logf("Q before: %.8f; alpha: %.2f; gamma: %.2f", result.QBefore, cfg.Training.Alpha, cfg.Training.Gamma)
					t.Logf("Q value: %.8f + %.2f*(%.8f + %.2f*%.8f - %.8f) = %.8f",
						result.QBefore, cfg.Training.Alpha, result.Reward, cfg.Training.Gamma,
						result.MaxNextQ, result.QBefore, result.QAfter)

					if result.State != state || result.NextState != wantState || env.State() != wantState || result.Action != action || !result.ActionTaken || !result.QUpdated {
						t.Fatalf("unexpected transition: got %+v -> %+v, want %+v", result.State, result.NextState, wantState)
					}
					near(t, env.distance, want.distance)
					near(t, result.TotalDistanceM, want.distance)
					near(t, result.Reward, want.reward)
					near(t, result.Components.Total(), want.reward)
					near(t, result.QBefore, want.qBefore)
					near(t, result.MaxNextQ, want.maxNextQ)
					near(t, result.QAfter, want.qAfter)
					near(t, q.Value(state, action), want.qAfter)
					if result.Done != (stepIndex == len(episode.steps)-1) {
						t.Fatal("episode must terminate exactly on the final prescribed action")
					}
					if !result.Done && result.Termination != Running {
						t.Fatal("unfinished episode must remain running")
					}
					previous = want.next
					totalReward += result.Reward
					last = result
				}
				near(t, totalReward, episode.totalReward)
				near(t, last.Components.DistanceError, episode.finalError)
				if last.Termination != episode.termination || env.reason != episode.termination || !env.done || env.isLoop != (episode.termination == Success) || env.steps != len(episode.steps) {
					t.Fatal("unexpected episode result")
				}
				wantTerminal, wantNoAction := -50.0, 0.0
				if episode.termination == Success {
					wantTerminal = 100
				}
				if episode.termination == NoAvailableAction {
					wantTerminal, wantNoAction = 0, -50
				}
				near(t, last.Components.Terminal, wantTerminal)
				near(t, last.Components.NoAction, wantNoAction)
				if len(env.Available()) != 0 {
					t.Fatal("finished episode must have no legal actions")
				}
				t.Log("Action mask after termination: available=[]")
				t.Logf("Episode %d result: distance=%.1f m; reward=%.8f; termination=%s",
					episodeIndex+1, env.distance, totalReward, last.Termination)
				pairs := 0
				for _, values := range q {
					pairs += len(values)
				}
				if pairs != episode.storedPairs {
					t.Fatalf("stored Q pairs: got %d, want %d", pairs, episode.storedPairs)
				}
				writeVerificationQTable(t, tc.scenario, episodeIndex+1, q)
				if episodeIndex == 0 {
					failedState, failedAction, failedQ = last.State, last.Action, last.QAfter
				}
			}
			// The failed terminal transition remains learned after episode 2.
			near(t, q.Value(failedState, failedAction), failedQ)
		})
	}
}

// Display only; state construction is implemented by Environment.State in environment.go.
func verificationState(s State) string {
	text := fmt.Sprintf("node %d", s.CurrentNode)
	if s.Scenario != ScenarioA {
		text += fmt.Sprintf(", bin %d", s.ProgressBin)
	}
	if s.Scenario == ScenarioC {
		text += fmt.Sprintf(", previous %d", s.PreviousNode)
	}
	return "(" + text + ")"
}

// Assert an independently listed legal set, then explain rejected raw edges.
// Core: environment.go: Environment.Available supplies the actual legal actions.
// Core: action.go: NewActions/AccessAllowed filter way and endpoint access first.
// The reason switch below repeats selected core conditions for log text ONLY;
// these reasons are inferred by this test, not returned by the core or asserted here.
func logVerificationMask(t *testing.T, e *Environment, want string) {
	t.Helper()
	var legal []ActionID
	var labels, blocked []string
	for _, edge := range e.Available() {
		id := ActionID{NextNode: edge.To, Key: edge.Key}
		legal = append(legal, id)
		labels = append(labels, fmt.Sprintf("(%d,%d)", edge.To, edge.Key))
	}
	if got := strings.Join(labels, " "); got != want {
		t.Fatalf("action mask at %s: got [%s], want [%s]", verificationState(e.State()), got, want)
	}
	for _, edge := range e.graph.graph.OutgoingEdges(e.current) {
		if slices.Contains(legal, ActionID{NextNode: edge.To, Key: edge.Key}) {
			continue
		}
		// Logging references: action.go: NewActions (access), environment.go: Available (closing/revisit).
		reason := "revisit not yet allowed"
		switch {
		case !AccessAllowed(edge.Access):
			reason = fmt.Sprintf("way access=%s", edge.Access)
		case !AccessAllowed(e.graph.graph.Nodes[edge.To].Tags.Get("access")):
			reason = fmt.Sprintf("node access=%s", e.graph.graph.Nodes[edge.To].Tags.Get("access"))
		case edge.To == e.graph.startNode:
			reason = fmt.Sprintf("closing distance %.1f outside %.0f-%.0f m", e.distance+edge.LengthM, e.config.MinDistanceM(), e.config.MaxDistanceM())
		case e.visits[edge.To] >= e.config.MaxNodeVisits:
			reason = "node visit limit"
		}
		blocked = append(blocked, fmt.Sprintf("(%d,%d): %s", edge.To, edge.Key, reason))
	}
	t.Logf("Action mask at node %d: available=[%s]; blocked=[%s]", e.current, strings.Join(labels, " "), strings.Join(blocked, "; "))
}

// Test-only CSV/log export; Q values were already updated by TrainStep in agent.go.
// Snapshot only stored pairs; absent pairs still have Q=0. CSV keeps full precision.
func writeVerificationQTable(t *testing.T, scenario Scenario, episode int, q QTable) {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "playground", "verification", string(scenario)))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	writer := csv.NewWriter(&text)
	if err := writer.Write([]string{"scenario", "state_node", "progress_bin", "previous_node", "next_node", "key", "q_value"}); err != nil {
		t.Fatal(err)
	}
	states := make([]State, 0, len(q))
	for state := range q {
		states = append(states, state)
	}
	slices.SortFunc(states, func(a, b State) int {
		if order := cmp.Compare(a.CurrentNode, b.CurrentNode); order != 0 {
			return order
		}
		if order := cmp.Compare(a.ProgressBin, b.ProgressBin); order != 0 {
			return order
		}
		return cmp.Compare(a.PreviousNode, b.PreviousNode)
	})
	t.Logf("Q-table after episode %d (stored pairs; absent pairs = 0):", episode)
	for _, state := range states {
		actions := make([]ActionID, 0, len(q[state]))
		for action := range q[state] {
			actions = append(actions, action)
		}
		slices.SortFunc(actions, func(a, b ActionID) int {
			if order := cmp.Compare(a.NextNode, b.NextNode); order != 0 {
				return order
			}
			return cmp.Compare(a.Key, b.Key)
		})
		for _, action := range actions {
			bin, previous := "", ""
			if scenario != ScenarioA {
				bin = strconv.Itoa(state.ProgressBin)
			}
			if scenario == ScenarioC {
				previous = strconv.Itoa(state.PreviousNode)
			}
			value := q.Value(state, action)
			row := []string{string(scenario), strconv.Itoa(state.CurrentNode), bin, previous,
				strconv.Itoa(action.NextNode), strconv.Itoa(action.Key), strconv.FormatFloat(value, 'g', -1, 64)}
			if err := writer.Write(row); err != nil {
				t.Fatal(err)
			}
			t.Logf("Q(%s, action (%d,%d)) = %.8f", verificationState(state), action.NextNode, action.Key, value)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, fmt.Sprintf("episode_%d_q_table.csv", episode))
	if err := os.WriteFile(path, []byte(text.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("CSV: %s", path)
}
