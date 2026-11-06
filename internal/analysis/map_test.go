package analysis

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
	"github.com/davidimannuel/go-running-route-q-learning/internal/qlearning"
)

func readMapData(t *testing.T, body string) mapData {
	t.Helper()
	const start = `<script id="road-data" type="application/json">`
	_, tail, ok := strings.Cut(body, start)
	if !ok {
		t.Fatal("missing map payload")
	}
	payload, _, ok := strings.Cut(tail, "</script>")
	if !ok {
		t.Fatal("unterminated data script")
	}
	var data mapData
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		t.Fatalf("invalid embedded JSON: %v: %.200s", err, payload)
	}
	return data
}

func TestMapToHTMLPreservesGraphAndScores(t *testing.T) {
	g := analysisTestGraph(t)
	before := sortedEdges(g)
	var body bytes.Buffer
	if err := MapToHTML(&body, g, -7.282187, 112.709282); err != nil {
		t.Fatal(err)
	}
	data := readMapData(t, body.String())
	if len(data.Nodes) != 4 || len(data.Edges) != 6 || len(data.Segments) != 2 || data.Center != [2]float64{-7.282187, 112.709282} {
		t.Fatalf("unexpected map sizes: nodes=%d edges=%d segments=%d center=%v", len(data.Nodes), len(data.Edges), len(data.Segments), data.Center)
	}
	if len(data.Segments[0].Edges) != 4 || len(data.Segments[1].Edges) != 2 {
		t.Fatalf("lost parallel/reverse edges: %+v", data.Segments)
	}
	seen := make(map[[3]string]bool)
	for _, e := range data.Edges {
		id := [3]string{e.From, e.To, strconv.Itoa(e.Key)}
		if seen[id] {
			t.Fatal("duplicate edge identity")
		}
		seen[id] = true
		if e.WayID == "100" {
			checkScores(t, e.RoadScores, qlearning.RoadScores{HighwayScore: 1, SurfaceScore: 1, WidthScore: 1, ComfortScore: 1})
		} else {
			checkScores(t, e.RoadScores, qlearning.RoadScores{HighwayScore: -0.5, SurfaceScore: -0.5, WidthScore: -0.5, ComfortScore: -0.5})
		}
	}
	if !reflect.DeepEqual(before, sortedEdges(g)) {
		t.Fatal("map export changed graph edges")
	}
	var again bytes.Buffer
	if err := MapToHTML(&again, g, -7.282187, 112.709282); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body.Bytes(), again.Bytes()) {
		t.Fatal("map output is not deterministic")
	}
}

func TestMapEscapesOSMTextAndPreservesLargeIDs(t *testing.T) {
	const id = 9_007_199_254_740_993
	const attack = `</script><script>alert("OSM")</script><img onerror=alert(1)>`
	g := osm.NewGraph()
	if err := g.LoadOSM(osm.OSMData{Elements: []osm.Element{
		{Type: osm.TypeNode, ID: id, Lat: 1, Lon: 2}, {Type: osm.TypeNode, ID: id + 1, Lat: 1, Lon: 2.01},
		{Type: osm.TypeWay, ID: 12, Nodes: []int{id, id + 1}, Tags: osm.Tags{"name": attack, "width": attack}},
	}}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := MapToHTML(&out, g, 1, 2); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), attack) {
		t.Fatal("OSM text can escape the embedded script")
	}
	data := readMapData(t, out.String())
	if data.Nodes[0].ID != "9007199254740993" || data.Edges[0].Name != attack || data.Edges[0].Width != attack {
		t.Fatal("source text or ID precision was lost")
	}
}

func TestMapEmptyGraphAndErrors(t *testing.T) {
	var out bytes.Buffer
	if err := MapToHTML(&out, osm.NewGraph(), 0, 0); err != nil {
		t.Fatal(err)
	}
	data := readMapData(t, out.String())
	if data.Nodes == nil || data.Edges == nil || data.Segments == nil {
		t.Fatal("empty arrays must not become null in JavaScript")
	}
	for _, point := range [][2]float64{{91, 0}, {0, 181}, {math.NaN(), 0}} {
		if err := MapToHTML(&bytes.Buffer{}, osm.NewGraph(), point[0], point[1]); err == nil {
			t.Fatalf("invalid input location accepted: %v", point)
		}
	}
	invalidCoordinates := osm.NewGraph()
	invalidCoordinates.Nodes[1] = osm.Element{Lat: 91}
	missingDestination := analysisTestGraph(t)
	delete(missingDestination.Nodes, 2)
	// Deliberately corrupt read-only views only to exercise export validation.
	missingSource := analysisTestGraph(t)
	missingSource.OutgoingEdges(1)[0].From = 99
	invalidLength := analysisTestGraph(t)
	invalidLength.OutgoingEdges(1)[0].LengthM = math.NaN()
	for _, g := range []*osm.Graph{invalidCoordinates, missingDestination, missingSource, invalidLength} {
		if err := MapToHTML(&bytes.Buffer{}, g, 0, 0); err == nil {
			t.Fatal("invalid graph accepted")
		}
	}
	sentinel := errors.New("writer unavailable")
	if err := MapToHTML(failingCSVWriter{sentinel}, osm.NewGraph(), 0, 0); !errors.Is(err, sentinel) {
		t.Fatalf("writer error was lost: %v", err)
	}
}

func analysisTestGraph(t testing.TB) *osm.Graph {
	t.Helper()
	graph := osm.NewGraph()
	if err := graph.LoadOSM(osm.OSMData{Elements: []osm.Element{
		{Type: osm.TypeNode, ID: 1}, {Type: osm.TypeNode, ID: 2},
		{Type: osm.TypeNode, ID: 3}, {Type: osm.TypeNode, ID: 4},
		{Type: osm.TypeWay, ID: 100, Nodes: []int{1, 2, 3}, Tags: osm.Tags{
			"name": "Main Street", "highway": "residential", "surface": "asphalt", "width": "4", "access": "permissive",
		}},
		{Type: osm.TypeWay, ID: 101, Nodes: []int{1, 2}},
	}}); err != nil {
		t.Fatal(err)
	}
	return graph
}

func checkScores(t *testing.T, got, want qlearning.RoadScores) {
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
