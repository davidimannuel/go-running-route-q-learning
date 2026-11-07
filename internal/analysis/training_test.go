package analysis

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
	"github.com/davidimannuel/go-running-route-q-learning/internal/qlearning"
)

func TestQTableRoundTripEmptyZeroAndErrors(t *testing.T) {
	s := qlearning.State{Scenario: qlearning.ScenarioC, CurrentNode: 0, PreviousNode: -1, ProgressBin: 0}
	table := qlearning.QTable{s: {{NextNode: 0, Key: 0}: 0, {NextNode: 12345678901, Key: 10}: -3.5},
		{Scenario: qlearning.ScenarioB, CurrentNode: 5, PreviousNode: -1}: {}}
	var buf bytes.Buffer
	if err := QTableToCSV(&buf, table); err != nil {
		t.Fatal(err)
	}
	actual, err := ReadQTable(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, table) {
		t.Fatalf("table roundtrip: %v", actual)
	}
	for _, content := range []string{"bad\n", strings.Join(qHeader, ",") + "\nC,0,-1,0,1,0,NaN\n", strings.Join(qHeader, ",") + "\nC,0,-1,0,1,0,2\nC,0,-1,0,1,0,3\n"} {
		if _, err := ReadQTable(strings.NewReader(content)); err == nil {
			t.Fatal("invalid Q-table accepted")
		}
	}
	expected := errors.New("disk full")
	if err := QTableToCSV(failingCSVWriter{expected}, table); !errors.Is(err, expected) {
		t.Fatalf("writer error=%v", err)
	}
}

func TestRouteMapAllSeedsEscapingAndFailure(t *testing.T) {
	graph := osm.NewGraph()
	hostile := `</script><script>alert("OSM")</script>`
	if err := graph.LoadOSM(osm.OSMData{Version: 0.6, Elements: []osm.Element{
		{Type: osm.TypeNode, ID: 0, Lat: 0, Lon: 0}, {Type: osm.TypeNode, ID: 1, Lat: 0, Lon: 0.01},
		{Type: osm.TypeWay, ID: 10, Nodes: []int{0, 1}, Tags: osm.Tags{"name": hostile}},
		{Type: osm.TypeNode, ID: 2, Lat: 0.01, Lon: 0.01},
		{Type: osm.TypeWay, ID: 20, Nodes: []int{1, 2}, Tags: osm.Tags{"access": "private"}},
	}}); err != nil {
		t.Fatal(err)
	}
	edge := &graph.OutgoingEdges(0)[0]
	trials := []RouteTrial{
		{Metrics: qlearning.Metrics{Scenario: qlearning.ScenarioB, Seed: 42, TerminationReason: qlearning.NoAvailableAction, MeanComfort: math.NaN(), OverlapRatio: math.NaN(), ReturnProgressRatio: math.NaN()},
			Steps: []qlearning.StepResult{{Transition: qlearning.Transition{ActionTaken: true, Edge: edge}}}},
		{Metrics: qlearning.Metrics{Scenario: qlearning.ScenarioB, Seed: 9007199254740993, IsLoop: true, AbsoluteDistanceErrorM: 10}},
	}
	var buf bytes.Buffer
	if err := RoutesToHTML(&buf, graph, 0, 0, 0, 0, trials, qlearning.DefaultRewardConfig().Road); err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	if strings.Contains(html, hostile) {
		t.Fatal("unescaped OSM popup text")
	}
	match := regexp.MustCompile(`const routeData = (.*);`).FindStringSubmatch(html)
	if len(match) != 2 {
		t.Fatal("embedded route data missing")
	}
	var payload struct {
		Selected int
		Trials   []routeTrialData
		Graph    mapData
	}
	if err := json.Unmarshal([]byte(match[1]), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Selected != 1 || len(payload.Trials) != 2 || payload.Trials[1].Seed != "9007199254740993" {
		t.Fatalf("payload=%+v", payload)
	}
	if len(payload.Graph.Nodes) != 3 || len(payload.Graph.Edges) != 4 || len(payload.Graph.Segments) != 2 {
		t.Fatalf("route inspection must include the surrounding graph: %+v", payload.Graph)
	}
	privateEdges := 0
	for _, edge := range payload.Graph.Edges {
		if edge.Access == "private" {
			privateEdges++
		}
		checkScores(t, edge.RoadScores, qlearning.RoadScores{HighwayScore: -0.5, SurfaceScore: -0.5, WidthScore: -0.5, ComfortScore: -0.5})
	}
	if privateEdges != 2 {
		t.Fatal("inspection must retain blocked roads without adding them to the route")
	}
	if payload.Trials[0].Segments[0].Name != hostile || payload.Trials[0].IsLoop {
		t.Fatal("failed route/OSM text lost")
	}
	for _, want := range []string{"textContent", "START", "FINISH", "tile.openstreetmap.org", "Failed"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if err := RoutesToHTML(failingCSVWriter{errors.New("full")}, graph, 0, 0, 0, 0, trials, qlearning.DefaultRewardConfig().Road); err == nil {
		t.Fatal("map writer error ignored")
	}
	if err := RoutesToHTML(&buf, graph, 0, 0, 0, 0, nil, qlearning.DefaultRewardConfig().Road); err == nil {
		t.Fatal("empty trials accepted")
	}
}
