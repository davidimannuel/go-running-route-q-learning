package analysis

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
	"github.com/davidimannuel/go-running-route-q-learning/internal/qlearning"
)

func TestConfiguredScoresAndRecordedReport(t *testing.T) {
	g := analysisTestGraph(t)
	scores := qlearning.DefaultRewardConfig().Road
	scores.Highway.Priority = 2
	var csv, html bytes.Buffer
	if err := EdgeToCSV(&csv, g, scores); err != nil {
		t.Fatal(err)
	}
	if err := MapToHTML(&html, g, 0, 0, scores); err != nil {
		t.Fatal(err)
	}
	data := readMapData(t, html.String())
	found := false
	for _, edge := range data.Edges {
		if edge.WayID == "100" {
			found = true
			checkScores(t, edge.RoadScores, qlearning.RoadScores{HighwayScore: 2, SurfaceScore: 1, WidthScore: 1, ComfortScore: 1.55})
		}
	}
	if !found || !strings.Contains(csv.String(), ",2,1,1,1.55") {
		t.Fatal("CSV/map used default scores")
	}
	meta := map[string]any{"status": "completed", "resolved_seeds": []uint64{18446744073709451615}, "config": map[string]any{"scenario": "B", "reward": scores}, "note": "</pre><script>bad()</script>"}
	html.Reset()
	if err := ReportToHTML(&html, "Training details", "training", meta); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html.String(), "18446744073709451615") || strings.Contains(html.String(), "</pre><script>bad()") {
		t.Fatal("report lost seed precision or text escaping")
	}
}

func TestSecondaryScoresAgreeAcrossCSVAndMaps(t *testing.T) {
	graph := osm.NewGraph()
	if err := graph.LoadOSM(osm.OSMData{Elements: []osm.Element{
		{Type: osm.TypeNode, ID: 1}, {Type: osm.TypeNode, ID: 2, Lon: 0.001},
		{Type: osm.TypeWay, ID: 100, Nodes: []int{1, 2}, Tags: osm.Tags{"highway": "secondary"}},
	}}); err != nil {
		t.Fatal(err)
	}
	scores := qlearning.DefaultRewardConfig().Road
	priority := 0.5
	scores.Highway.SecondaryPriority = &priority
	want := qlearning.RoadScores{HighwayScore: 0.5, SurfaceScore: -0.5, WidthScore: -0.5, ComfortScore: 0.05}
	var csv, html bytes.Buffer
	if err := EdgeToCSV(&csv, graph, scores); err != nil {
		t.Fatal(err)
	}
	for _, row := range readCSV(t, csv.String())[1:] {
		highway, err := strconv.ParseFloat(row[10], 64)
		if err != nil || highway != 0.5 {
			t.Fatalf("CSV highway score=%q", row[10])
		}
		comfort, err := strconv.ParseFloat(row[13], 64)
		if err != nil {
			t.Fatal(err)
		}
		checkScores(t, qlearning.RoadScores{HighwayScore: highway, SurfaceScore: -0.5, WidthScore: -0.5, ComfortScore: comfort}, want)
	}
	if err := MapToHTML(&html, graph, 0, 0, scores); err != nil {
		t.Fatal(err)
	}
	for _, edge := range readMapData(t, html.String()).Edges {
		checkScores(t, edge.RoadScores, want)
	}
	html.Reset()
	edge := &graph.OutgoingEdges(1)[0]
	trials := []RouteTrial{{
		Metrics: qlearning.Metrics{Scenario: qlearning.ScenarioB},
		Steps:   []qlearning.StepResult{{Transition: qlearning.Transition{ActionTaken: true, Edge: edge}}},
	}}
	if err := RoutesToHTML(&html, graph, 1, 0, 0, 0, trials, scores); err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`const routeData = (.*);`).FindStringSubmatch(html.String())
	if len(match) != 2 {
		t.Fatal("route map data missing")
	}
	var payload routeMapData
	if err := json.Unmarshal([]byte(match[1]), &payload); err != nil {
		t.Fatal(err)
	}
	checkScores(t, payload.Trials[0].Segments[0].Scores, want)
	for _, edge := range payload.Graph.Edges {
		checkScores(t, edge.RoadScores, want)
	}
}
