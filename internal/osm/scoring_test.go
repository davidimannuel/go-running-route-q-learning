package osm

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
)

func TestScoreRoadManualExamples(t *testing.T) {
	for _, tc := range []struct {
		highway, surface, width string
		want                    RoadScores
	}{
		{"residential", "asphalt", "3 m", RoadScores{1, 1, 1, 1}},
		{"service", "", "", RoadScores{-1, -0.5, -0.5, -0.775}},
		{"", "", "", RoadScores{-0.5, -0.5, -0.5, -0.5}},
		{"residential;service", "paved", "3,5", RoadScores{-1, 1, 1, -0.1}},
		{"tertiary", "unknown", "2", RoadScores{1, -1, -1, 0.1}},
		{"living_street", "", "2", RoadScores{1, -0.5, -1, 0.25}},
	} {
		widthM, valid := ParseWidthM(tc.width)
		checkScores(t, ScoreRoad(tc.highway, tc.surface, widthM, valid), tc.want)
	}
}

func checkScores(t *testing.T, got, want RoadScores) {
	t.Helper()
	for _, pair := range [][2]float64{
		{got.HighwayScore, want.HighwayScore}, {got.SurfaceScore, want.SurfaceScore},
		{got.WidthScore, want.WidthScore}, {got.ComfortScore, want.ComfortScore},
	} {
		if math.IsNaN(pair[0]) || math.Abs(pair[0]-pair[1]) > 1e-12 {
			t.Fatalf("scores=%+v; want %+v", got, want)
		}
	}
}

func TestScoresMatchReferencePython(t *testing.T) {
	// These frozen expectations were produced by the actual reference functions,
	// not by this Go mapping. Manual examples above provide an independent check.
	contents, err := os.ReadFile("testdata/scoring_reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Highway  string     `json:"highway"`
			Surface  string     `json:"surface"`
			Width    string     `json:"width"`
			Expected RoadScores `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(contents, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) < 30 {
		t.Fatal("reference cases are missing")
	}
	for _, tc := range fixture.Cases {
		t.Run(tc.Highway+"/"+tc.Surface+"/"+tc.Width, func(t *testing.T) {
			graph := NewGraph()
			if err := graph.LoadOSM(OSMData{Elements: []Element{
				{Type: TypeNode, ID: 1}, {Type: TypeNode, ID: 2, Lon: 0.001},
				{Type: TypeWay, ID: 100, Nodes: []int{1, 2}, Tags: Tags{"highway": tc.Highway, "surface": tc.Surface, "width": tc.Width}},
			}}); err != nil {
				t.Fatal(err)
			}
			for _, edge := range graph.sortedEdges() {
				checkScores(t, edge.Scores(), tc.Expected)
			}
		})
	}
}

func TestParseWidthM(t *testing.T) {
	for raw, want := range map[string]float64{
		"3": 3, " 3 M ": 3, "2,5 metres": 2.5, "3.00 meters": 3,
		"0.25 metre": 0.25, "2meter": 2, "04": 4,
	} {
		if got, ok := ParseWidthM(raw); !ok || got != want {
			t.Errorf("%q = %v, %v; want %v", raw, got, ok, want)
		}
	}
	for _, raw := range []string{"", "0", "-3", "+3", ".5", "3.", "3;4", "[3]", "['3']", "3 ft", "NaN", "Inf", "3e0", "1,2,3", strings.Repeat("9", 400)} {
		if got, ok := ParseWidthM(raw); ok || got != 0 {
			t.Errorf("%q unexpectedly parsed as %v", raw, got)
		}
	}
}

func TestEdgeScoresPreserveOriginalWidth(t *testing.T) {
	edge := OSMEdge{Highway: "residential", Surface: "asphalt", Width: 3, WidthValid: true, Tags: Tags{"width": "3 m"}, Access: "permissive"}
	checkScores(t, edge.Scores(), RoadScores{1, 1, 1, 1})
	if edge.Tags.Get("width") != "3 m" || edge.Width != 3 || !edge.WidthValid {
		t.Fatal("scoring modified the source tags")
	}
}

// An invalid flag must not accidentally turn a numeric zero into a narrow road.
// Scores use parsed fields only, even when raw text is available for display.
func TestScoresRespectParsedWidthValidity(t *testing.T) {
	edge := OSMEdge{Highway: "residential", Surface: "asphalt", Width: 3, Tags: Tags{"width": "3 m"}}
	checkScores(t, edge.Scores(), RoadScores{1, 1, -0.5, 0.775})
	for _, width := range []float64{0, -1, math.Inf(1), math.NaN()} {
		checkScores(t, ScoreRoad("residential", "asphalt", width, true), RoadScores{1, 1, -0.5, 0.775})
	}
}
