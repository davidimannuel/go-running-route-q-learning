package osm

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
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
	g := NewGraph()
	if err := g.LoadOSM(graphTestData(false)); err != nil {
		t.Fatal(err)
	}
	before := g.sortedEdges()
	var body bytes.Buffer
	if err := g.MapToHTML(&body, -7.282187, 112.709282); err != nil {
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
			checkScores(t, e.RoadScores, RoadScores{1, 1, 1, 1})
		} else {
			checkScores(t, e.RoadScores, RoadScores{-0.5, -0.5, -0.5, -0.5})
		}
	}
	if !reflect.DeepEqual(before, g.sortedEdges()) {
		t.Fatal("map export changed graph edges")
	}
	var again bytes.Buffer
	if err := g.MapToHTML(&again, -7.282187, 112.709282); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body.Bytes(), again.Bytes()) {
		t.Fatal("map output is not deterministic")
	}
}

func TestMapEscapesOSMTextAndPreservesLargeIDs(t *testing.T) {
	const id = 9_007_199_254_740_993
	const attack = `</script><script>alert("OSM")</script><img onerror=alert(1)>`
	g := NewGraph()
	if err := g.LoadOSM(OSMData{Elements: []Element{
		{Type: TypeNode, ID: id, Lat: 1, Lon: 2}, {Type: TypeNode, ID: id + 1, Lat: 1, Lon: 2.01},
		{Type: TypeWay, ID: 12, Nodes: []int{id, id + 1}, Tags: Tags{"name": attack, "width": attack}},
	}}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := g.MapToHTML(&out, 1, 2); err != nil {
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
	if err := NewGraph().MapToHTML(&out, 0, 0); err != nil {
		t.Fatal(err)
	}
	data := readMapData(t, out.String())
	if data.Nodes == nil || data.Edges == nil || data.Segments == nil {
		t.Fatal("empty arrays must not become null in JavaScript")
	}
	for _, point := range [][2]float64{{91, 0}, {0, 181}, {math.NaN(), 0}} {
		if err := NewGraph().MapToHTML(&bytes.Buffer{}, point[0], point[1]); err == nil {
			t.Fatalf("invalid input location accepted: %v", point)
		}
	}
	for _, g := range []*Graph{
		{Nodes: map[int]Element{1: {Lat: 91}}},
		{Nodes: map[int]Element{1: {}}, adjList: map[int][]OSMEdge{1: {{From: 1, To: 2}}}},
		{Nodes: map[int]Element{1: {}}, adjList: map[int][]OSMEdge{1: {{From: 2, To: 1}}}},
		{Nodes: map[int]Element{1: {}, 2: {}}, adjList: map[int][]OSMEdge{1: {{From: 1, To: 2, LengthM: math.NaN()}}}},
	} {
		if err := g.MapToHTML(&bytes.Buffer{}, 0, 0); err == nil {
			t.Fatal("invalid graph accepted")
		}
	}
	sentinel := errors.New("writer unavailable")
	if err := NewGraph().MapToHTML(failingCSVWriter{sentinel}, 0, 0); !errors.Is(err, sentinel) {
		t.Fatalf("writer error was lost: %v", err)
	}
}
