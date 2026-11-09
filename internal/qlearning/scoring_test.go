package qlearning

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
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
		widthM, valid := osm.ParseWidthM(tc.width)
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
			graph := osm.NewGraph()
			if err := graph.LoadOSM(osm.OSMData{Elements: []osm.Element{
				{Type: osm.TypeNode, ID: 1}, {Type: osm.TypeNode, ID: 2, Lon: 0.001},
				{Type: osm.TypeWay, ID: 100, Nodes: []int{1, 2}, Tags: osm.Tags{"highway": tc.Highway, "surface": tc.Surface, "width": tc.Width}},
			}}); err != nil {
				t.Fatal(err)
			}
			for nodeID := range graph.Nodes {
				for _, edge := range graph.OutgoingEdges(nodeID) {
					checkScores(t, ScoreEdge(edge), tc.Expected)
				}
			}
		})
	}
}

func TestEdgeScoresPreserveOriginalWidth(t *testing.T) {
	edge := osm.OSMEdge{Highway: "residential", Surface: "asphalt", Width: 3, WidthValid: true, Tags: osm.Tags{"width": "3 m"}, Access: "permissive"}
	checkScores(t, ScoreEdge(edge), RoadScores{1, 1, 1, 1})
	if edge.Tags.Get("width") != "3 m" || edge.Width != 3 || !edge.WidthValid {
		t.Fatal("scoring modified the source tags")
	}
}

// An invalid flag must not accidentally turn a numeric zero into a narrow road.
// Scores use parsed fields only, even when raw text is available for display.
func TestScoresRespectParsedWidthValidity(t *testing.T) {
	edge := osm.OSMEdge{Highway: "residential", Surface: "asphalt", Width: 3, Tags: osm.Tags{"width": "3 m"}}
	checkScores(t, ScoreEdge(edge), RoadScores{1, 1, -0.5, 0.775})
	for _, width := range []float64{0, -1, math.Inf(1), math.NaN()} {
		checkScores(t, ScoreRoad("residential", "asphalt", width, true), RoadScores{1, 1, -0.5, 0.775})
	}
}

func TestSecondaryHighwayPriority(t *testing.T) {
	cfg := DefaultRewardConfig().Road
	priority := 0.5
	cfg.Highway.SecondaryPriority = &priority
	for _, tc := range []struct {
		highway string
		want    RoadScores
	}{
		{"secondary", RoadScores{0.5, -0.5, -0.5, 0.05}},
		{" Secondary ", RoadScores{0.5, -0.5, -0.5, 0.05}},
		{"residential", RoadScores{1, -0.5, -0.5, 0.325}},
		{"tertiary", RoadScores{1, -0.5, -0.5, 0.325}},
		{"service", RoadScores{-1, -0.5, -0.5, -0.775}},
		{"secondary_link", RoadScores{-1, -0.5, -0.5, -0.775}},
		{"", RoadScores{-0.5, -0.5, -0.5, -0.5}},
		{"residential;secondary", RoadScores{0.5, -0.5, -0.5, 0.05}},
		{"secondary;service", RoadScores{-1, -0.5, -0.5, -0.775}},
		{"['residential', 'secondary']", RoadScores{0.5, -0.5, -0.5, 0.05}},
	} {
		t.Run(tc.highway, func(t *testing.T) {
			checkScores(t, cfg.ScoreRoad(tc.highway, "", 0, false), tc.want)
		})
	}
	// Only highway=secondary receives this preference, not a surface tag.
	checkScores(t, cfg.ScoreRoad("secondary", "secondary", 0, false), RoadScores{0.5, -1, -0.5, -0.1})
	checkScores(t, cfg.ScoreRoad("secondary", "asphalt", 3, true), RoadScores{0.5, 1, 1, 0.725})
}

func TestSecondaryHighwayLegacyFallbackAndZero(t *testing.T) {
	cfg := DefaultRewardConfig().Road
	cfg.Highway.Other = -2
	checkScores(t, cfg.ScoreRoad("secondary", "", 0, false), RoadScores{-2, -0.5, -0.5, -1.325})
	zero := 0.0
	cfg.Highway.SecondaryPriority = &zero
	checkScores(t, cfg.ScoreRoad("secondary", "", 0, false), RoadScores{0, -0.5, -0.5, -0.225})
}
