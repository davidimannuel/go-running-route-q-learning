package qlearning

import (
	"math"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
)

// Synthetic lengths deliberately differ from coordinate distances. This makes
// boundary/reward expectations independent of the production distance formula.
func testPreparedGraph(t *testing.T, pairs [][3]int) *PreparedGraph {
	t.Helper()
	nodes := map[int]bool{0: true}
	data := osm.OSMData{Version: 0.6}
	for i, pair := range pairs {
		nodes[pair[0]], nodes[pair[1]] = true, true
		data.Elements = append(data.Elements, osm.Element{Type: osm.TypeWay, ID: i + 100, Nodes: []int{pair[0], pair[1]}, Tags: osm.Tags{"highway": "residential", "surface": "asphalt", "width": "3"}})
	}
	for id := range nodes {
		data.Elements = append(data.Elements, osm.Element{Type: osm.TypeNode, ID: id, Lat: 0, Lon: float64(id) * 0.001})
	}
	graph := osm.NewGraph()
	if err := graph.LoadOSM(data); err != nil {
		t.Fatal(err)
	}
	// Test-only fixture preparation, before creating immutable PreparedGraph/Actions.
	for id := range nodes {
		for i := range graph.OutgoingEdges(id) {
			edge := &graph.OutgoingEdges(id)[i]
			edge.LengthM = float64(pairs[edge.WayID-100][2])
		}
	}
	p, err := NewPreparedGraph(graph, 0)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func testEnvironment(t *testing.T, p *PreparedGraph, scenario Scenario) *Environment {
	t.Helper()
	env, err := NewEnvironment(p, scenario, DefaultConfig().Environment)
	if err != nil {
		t.Fatal(err)
	}
	return env
}
func near(t *testing.T, got, want float64) {
	t.Helper()
	if math.IsNaN(got) || math.Abs(got-want) > 1e-10 {
		t.Fatalf("got %.14g, want %.14g", got, want)
	}
}
func actionIDs(edges []*osm.OSMEdge) []ActionID {
	var ids []ActionID
	for _, e := range edges {
		ids = append(ids, ActionID{e.To, e.Key})
	}
	return ids
}

func TestStateScenariosResetAndZero(t *testing.T) {
	p := testPreparedGraph(t, [][3]int{{0, 1, 100}})
	for _, s := range []Scenario{ScenarioA, ScenarioB, ScenarioC} {
		e := testEnvironment(t, p, s)
		reset := e.State()
		if reset.CurrentNode != 0 || reset.PreviousNode != -1 || reset.ProgressBin != 0 {
			t.Fatalf("reset=%+v", reset)
		}
		e.current, e.previous, e.distance = 1, 0, 2750
		got := e.State()
		wantBin := 8
		if s == ScenarioA {
			wantBin = 0
		}
		if got.ProgressBin != wantBin {
			t.Fatalf("state=%+v", got)
		}
		if s == ScenarioC && got.PreviousNode != 0 {
			t.Fatalf("valid previous node zero lost: %+v", got)
		}
		if s != ScenarioC && got.PreviousNode != -1 {
			t.Fatal("inactive previous node must remain -1")
		}
		e.distance = 5500
		if s != ScenarioA && e.State().ProgressBin != 15 {
			t.Fatal("upper boundary must be bin 15")
		}
		e.Reset()
		if e.State() != reset || len(e.path) != 0 || e.visits[0] != 1 || e.closingAvailable {
			t.Fatal("reset leaked episode state")
		}
	}
}

func TestMaskRevisitClosingAndParallelActions(t *testing.T) {
	p := testPreparedGraph(t, [][3]int{{0, 1, 500}, {1, 2, 100}, {1, 2, 100}, {1, 3, 100}})
	e := testEnvironment(t, p, ScenarioC)
	e.current = 1
	e.visits[1] = 1
	e.visits[2] = 1
	e.distance = 2499
	if got := actionIDs(e.Available()); !reflect.DeepEqual(got, []ActionID{{3, 0}}) {
		t.Fatalf("early mask=%v", got)
	}
	e.distance = 2500
	if got := actionIDs(e.Available()); !reflect.DeepEqual(got, []ActionID{{3, 0}, {2, 0}, {2, 1}}) {
		t.Fatalf("revisit mask=%v", got)
	}
	e.visits[2] = 2
	e.distance = 4000
	if got := actionIDs(e.Available()); !reflect.DeepEqual(got, []ActionID{{3, 0}, {0, 0}}) {
		t.Fatalf("closing mask=%v", got)
	}
	if !e.closingAvailable {
		t.Fatal("closing opportunity not latched")
	}
	e.distance = 5001
	e.Available()
	if !e.closingAvailable {
		t.Fatal("closing latch was cleared")
	}
	e.Reset()
	if e.closingAvailable {
		t.Fatal("closing opportunity leaked across episodes")
	}
	// Overshooting a non-start node remains an action; failure happens in Step.
	e.current = 1
	e.distance = 5499
	if len(e.Available()) != 3 {
		t.Fatal("non-start overshoot edges were incorrectly pruned")
	}
}

func TestLoopBoundsTerminalPriorityAndClosingExemption(t *testing.T) {
	for _, distance := range []int{4500, 5500} {
		p := testPreparedGraph(t, [][3]int{{0, 1, 2500}, {1, 2, distance - 3000}, {2, 0, 500}})
		cfg := DefaultConfig().Environment
		cfg.MaxSteps = 3
		e, err := NewEnvironment(p, ScenarioC, cfg)
		if err != nil {
			t.Fatal(err)
		}
		for _, next := range []int{1, 2, 0} {
			transition, err := e.Step(ActionID{next, 0})
			if err != nil {
				t.Fatal(err)
			}
			if next == 0 {
				if !transition.Done || !e.isLoop || transition.Termination != Success {
					t.Fatalf("terminal=%+v", transition)
				}
				near(t, transition.Components.Terminal, 100)
				near(t, transition.Components.Direction, 0)
				near(t, transition.Components.Revisit, 0)
			}
		}
		near(t, e.distance, float64(distance))
		if e.revisits != 0 {
			t.Fatal("closing counted as ordinary revisit")
		}
		if _, err := e.Step(ActionID{1, 0}); err == nil {
			t.Fatal("step after terminal accepted")
		}
	}
}

func TestFailureBoundariesAndIllegalActions(t *testing.T) {
	p := testPreparedGraph(t, [][3]int{{0, 1, 100}, {1, 2, 100}})
	e := testEnvironment(t, p, ScenarioB)
	before := e.State()
	if _, err := e.Step(ActionID{1, 99}); err == nil || e.State() != before {
		t.Fatal("illegal parallel key changed state")
	}
	e.current = 1
	e.distance = 5500
	tr, err := e.Step(ActionID{2, 0})
	if err != nil {
		t.Fatal(err)
	}
	if tr.Termination != OverDistance {
		t.Fatalf("termination=%s", tr.Termination)
	}
	near(t, tr.Components.Terminal, -50)
	near(t, tr.Components.Direction, 0)
	cfg := DefaultConfig().Environment
	cfg.MaxSteps = 1
	e, err = NewEnvironment(p, ScenarioA, cfg)
	if err != nil {
		t.Fatal(err)
	}
	tr, err = e.Step(ActionID{1, 0})
	if err != nil {
		t.Fatal(err)
	}
	if tr.Termination != StepLimit {
		t.Fatal("step limit not applied")
	}
	near(t, tr.Components.Terminal, -50)
}

func TestDirectionPhaseCrossingAndRevisitReward(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		distance, after, want float64
	}{
		{"early approach", 1000, 50, -0.3}, {"early away", 1000, 150, 0},
		{"crossing approach still early penalty", 2490, 50, -0.3},
		{"return approach", 2500, 50, 0.3}, {"return away", 2500, 150, -0.3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testPreparedGraph(t, [][3]int{{0, 1, 100}, {1, 2, 100}, {2, 3, 100}})
			p.distances[1], p.distances[2] = 100, tc.after
			e := testEnvironment(t, p, ScenarioB)
			e.current = 1
			e.distance = tc.distance
			e.visits[1] = 1
			tr, err := e.Step(ActionID{2, 0})
			if err != nil {
				t.Fatal(err)
			}
			near(t, tr.Components.Direction, tc.want)
		})
	}
	p := testPreparedGraph(t, [][3]int{{0, 1, 100}, {1, 2, 100}})
	e := testEnvironment(t, p, ScenarioB)
	e.current = 1
	e.distance = 2500
	e.visits[1] = 1
	e.visits[2] = 1
	tr, err := e.Step(ActionID{2, 0})
	if err != nil {
		t.Fatal(err)
	}
	near(t, tr.Components.Revisit, -10)
	if e.revisits != 1 {
		t.Fatal("revisit not counted")
	}
}

func TestBellmanManualLegalNonzeroBootstrapAndAccess(t *testing.T) {
	p := testPreparedGraph(t, [][3]int{{0, 1, 100}, {1, 2, 100}, {2, 3, 100}})
	// Test-only tag setup, then rebuild the immutable prepared graph.
	p.graph.OutgoingEdges(0)[0].Access = " permissive "
	var err error
	p, err = NewPreparedGraph(p.graph, 0)
	if err != nil {
		t.Fatal(err)
	}
	e := testEnvironment(t, p, ScenarioB)
	q := make(QTable)
	state := e.State()
	id := ActionID{1, 0}
	q[state] = map[ActionID]float64{id: 2}
	next := State{Scenario: ScenarioB, CurrentNode: 1, PreviousNode: -1, ProgressBin: 0}
	q[next] = map[ActionID]float64{{NextNode: 2, Key: 0}: 4, {NextNode: 0, Key: 0}: 1000}
	got, err := TrainStep(e, q, rand.New(rand.NewPCG(0, 0)), DefaultConfig().Training, 0, &id)
	if err != nil {
		t.Fatal(err)
	}
	// Independently: r=1-0.1=0.9; target=0.9+0.95*4=4.7;
	// error=4.7-2=2.7; updated Q=2+0.15*2.7=2.405.
	near(t, got.Reward, 0.9)
	near(t, got.Components.Access, -0.1)
	near(t, got.MaxNextQ, 4)
	near(t, got.TDTarget, 4.7)
	near(t, got.TDError, 2.7)
	near(t, got.QAfter, 2.405)
	near(t, q.Value(state, id), 2.405)
	if !got.QUpdated || got.Done {
		t.Fatalf("flags=%+v", got)
	}
}

func TestDeadEndPenaltyBeforeUpdateAndInitialNoAction(t *testing.T) {
	p := testPreparedGraph(t, [][3]int{{0, 1, 100}})
	e := testEnvironment(t, p, ScenarioA)
	q := make(QTable)
	id := ActionID{1, 0}
	state := e.State()
	q[state] = map[ActionID]float64{id: 2}
	q[State{Scenario: ScenarioA, CurrentNode: 1, PreviousNode: -1}] = map[ActionID]float64{{NextNode: 0, Key: 0}: 1000}
	got, err := TrainStep(e, q, rand.New(rand.NewPCG(1, 0)), DefaultConfig().Training, 0, &id)
	if err != nil {
		t.Fatal(err)
	}
	// No legal return at 200 m: r=1-50=-49; terminal bootstrap=0;
	// Q=2+0.15*(-49-2)=-5.65. No second terminal -50 is added.
	near(t, got.RewardStep, 1)
	near(t, got.StopPenalty, -50)
	near(t, got.Reward, -49)
	near(t, got.Components.Terminal, 0)
	near(t, got.Components.Total(), -49)
	near(t, got.MaxNextQ, 0)
	near(t, got.QAfter, -5.65)
	if !got.Done || got.Termination != NoAvailableAction || !got.QUpdated {
		t.Fatalf("result=%+v", got)
	}
	e = testEnvironment(t, testPreparedGraph(t, nil), ScenarioA)
	q = make(QTable)
	got, err = TrainStep(e, q, rand.New(rand.NewPCG(1, 0)), DefaultConfig().Training, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.ActionTaken || got.QUpdated || len(q) != 0 {
		t.Fatal("initial dead end updated an absent action")
	}
	near(t, got.Reward, -50)
	metrics := e.Metrics(-50)
	if !math.IsNaN(metrics.MeanComfort) || !math.IsNaN(metrics.OverlapRatio) || !math.IsNaN(metrics.ReturnProgressRatio) {
		t.Fatal("undefined metrics must remain NaN")
	}
}

func TestOverlapCountsGeometryNotDistanceOrDirectedKeys(t *testing.T) {
	p := testPreparedGraph(t, [][3]int{{0, 1, 1500}, {1, 2, 1000}, {2, 3, 1000}, {3, 0, 1000}, {0, 1, 1500}})
	e := testEnvironment(t, p, ScenarioB)
	// Five traversals: four unique physical segments. Reverse/parallel copies
	// with identical geometry must share a physical road ID.

	find := func(from, to, key int) *osm.OSMEdge {
		for _, a := range p.actions.Available(from) {
			if a.To == to && a.Key == key {
				return a
			}
		}
		t.Fatal("edge missing")
		return nil
	}
	for _, edge := range []*osm.OSMEdge{find(0, 1, 0), find(1, 2, 0), find(2, 3, 0), find(3, 0, 0), find(1, 0, 1)} {
		e.path = append(e.path, edge)
		e.roads[p.edges[edge].roadID] = struct{}{}
	}
	near(t, e.Metrics(0).OverlapRatio, 0.2)
}

func TestSuccessfulTerminalDoesNotBootstrap(t *testing.T) {
	p := testPreparedGraph(t, [][3]int{{0, 1, 2500}, {1, 2, 2000}, {2, 0, 500}})
	e := testEnvironment(t, p, ScenarioC)
	for _, next := range []int{1, 2} {
		if _, err := e.Step(ActionID{NextNode: next}); err != nil {
			t.Fatal(err)
		}
	}
	q := make(QTable)
	id := ActionID{NextNode: 0}
	q[e.State()] = map[ActionID]float64{id: 2}
	// This terminal state's large value must not enter the target.
	terminal := State{Scenario: ScenarioC, CurrentNode: 0, PreviousNode: 2, ProgressBin: 14}
	q[terminal] = map[ActionID]float64{{NextNode: 1}: 1000000}
	got, err := TrainStep(e, q, rand.New(rand.NewPCG(0, 0)), DefaultConfig().Training, 0, &id)
	if err != nil {
		t.Fatal(err)
	}
	// Independently: edge=500/100=5, success=100, no direction/revisit;
	// target=105, Q=2+0.15*(105-2)=17.45.
	near(t, got.Reward, 105)
	near(t, got.MaxNextQ, 0)
	near(t, got.TDTarget, 105)
	near(t, got.QAfter, 17.45)
	if got.Termination != Success || !got.Done || !got.QUpdated {
		t.Fatalf("terminal=%+v", got)
	}
	near(t, got.Components.Direction, 0)
	near(t, got.Components.Revisit, 0)
}

func TestActionsNumericDestinationAndKeyOrder(t *testing.T) {
	pairs := make([][3]int, 12)
	for i := range pairs {
		pairs[i] = [3]int{0, 1, 100}
	}
	pairs = append(pairs, [3]int{0, 2, 100})
	graph := testPreparedGraph(t, pairs).Graph()
	// Test-only permutation before constructing the Actions under test. The
	// destination 2 now comes first, followed by keys 11..0 to destination 1.
	edges := graph.OutgoingEdges(0)
	slices.Reverse(edges)
	before := slices.Clone(edges)
	candidates := NewActions(graph).Available(0)
	for key := 0; key < 12; key++ {
		if candidates[key].To != 1 || candidates[key].Key != key {
			t.Fatalf("candidate %d = (%d,%d), want (1,%d)", key, candidates[key].To, candidates[key].Key, key)
		}
	}
	if candidates[12].To != 2 || candidates[12].Key != 0 {
		t.Fatal("destination must take priority over key")
	}
	if !reflect.DeepEqual(before, graph.OutgoingEdges(0)) {
		t.Fatal("sorting candidates reordered graph storage")
	}
}
