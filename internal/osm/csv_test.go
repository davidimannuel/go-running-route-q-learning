package osm

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func readCSV(t *testing.T, data string) [][]string {
	t.Helper()
	rows, err := csv.NewReader(strings.NewReader(data)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestNodeToCSV(t *testing.T) {
	graph := NewGraph()
	graph.Nodes[30] = Element{ID: 30}
	graph.Nodes[9620836306] = Element{ID: 9620836306, Lat: -7.282187123456789, Lon: 112.70928212345678}
	graph.Nodes[10] = Element{ID: 10, Lat: -7.25, Lon: 112.5}
	var out bytes.Buffer
	if err := graph.NodeToCSV(&out); err != nil {
		t.Fatal(err)
	}
	rows := readCSV(t, out.String())
	if len(rows) != 4 || !reflect.DeepEqual(rows[0], []string{"node_id", "latitude", "longitude"}) {
		t.Fatalf("unexpected CSV: %v", rows)
	}
	for i, id := range []int{10, 30, 9620836306} {
		row := rows[i+1]
		lat, latErr := strconv.ParseFloat(row[1], 64)
		lon, lonErr := strconv.ParseFloat(row[2], 64)
		if row[0] != strconv.Itoa(id) || latErr != nil || lonErr != nil || lat != graph.Nodes[id].Lat || lon != graph.Nodes[id].Lon {
			t.Fatalf("node %d lost its identity or coordinate precision: %v", id, row)
		}
	}
}

func TestEdgeToCSV(t *testing.T) {
	tags := Tags{"width": "3;4"}
	graph := &Graph{adjList: map[int][]OSMEdge{
		2: {{From: 2, To: 1, Key: 0, WayID: 9, LengthM: 12.345678901234567}},
		1: {
			{From: 1, To: 3, Key: 0, WayID: 8, LengthM: 20.5, Name: "Road, \"A\"\nNorth", Highway: "residential", Surface: "asphalt", Tags: tags, Access: "permissive"},
			{From: 1, To: 2, Key: 2, WayID: 10, LengthM: 15, Width: 3, WidthValid: true, Tags: Tags{"width": "3 m"}},
			{From: 1, To: 2, Key: 1, WayID: 9, LengthM: 12.345678901234567},
		},
	}}
	want := [][]string{
		{"from", "to", "key", "way_id", "length_m", "name", "highway", "surface", "width", "access"},
		{"1", "2", "1", "9", "12.345678901234567", "", "", "", "", ""},
		{"1", "2", "2", "10", "15", "", "", "", "3 m", ""},
		{"1", "3", "0", "8", "20.5", "Road, \"A\"\nNorth", "residential", "asphalt", "3;4", "permissive"},
		{"2", "1", "0", "9", "12.345678901234567", "", "", "", "", ""},
	}
	for range 2 {
		var out bytes.Buffer
		if err := graph.EdgeToCSV(&out); err != nil {
			t.Fatal(err)
		}
		got := readCSV(t, out.String())
		if len(got) != len(want) {
			t.Fatalf("rows=%v; want %v", got, want)
		}
		for i := range want {
			if len(got[i]) != 14 || !reflect.DeepEqual(got[i][:10], want[i]) {
				t.Fatalf("row=%v; want prefix %v", got[i], want[i])
			}
		}
		if !reflect.DeepEqual(got[0][10:], []string{"highway_score", "surface_score", "width_score", "comfort_score"}) {
			t.Fatal("missing score columns")
		}
		// Expected signed scores are independent of the export implementation.
		for i, scores := range [][4]float64{{-0.5, -0.5, -0.5, -0.5}, {-0.5, -0.5, 1, -0.275}, {1, 1, -0.5, 0.775}, {-0.5, -0.5, -0.5, -0.5}} {
			for j, wantScore := range scores {
				value, err := strconv.ParseFloat(got[i+1][10+j], 64)
				if err != nil || math.Abs(value-wantScore) > 1e-12 {
					t.Fatalf("score=%v; want %v", got[i+1], scores)
				}
			}
		}
	}
	if graph.adjList[1][0].To != 3 || graph.adjList[1][1].Key != 2 {
		t.Fatal("CSV sorting changed adjacency order or action keys")
	}
}

type failingCSVWriter struct{ err error }

func (w failingCSVWriter) Write([]byte) (int, error) { return 0, w.err }

func TestCSVWriteErrors(t *testing.T) {
	for _, large := range []bool{false, true} {
		graph := NewGraph()
		if large {
			for id := range 500 {
				graph.Nodes[id] = Element{ID: id, Lat: 1, Lon: 2}
			}
			graph.adjList[1] = []OSMEdge{{From: 1, To: 2, Name: strings.Repeat("x", 5000)}}
		}
		for _, export := range []struct {
			name  string
			write func(io.Writer) error
		}{
			{name: "nodes", write: graph.NodeToCSV},
			{name: "edges", write: graph.EdgeToCSV},
		} {
			sentinel := errors.New("writer unavailable")
			if err := export.write(failingCSVWriter{err: sentinel}); !errors.Is(err, sentinel) {
				t.Fatalf("%s (large=%v): writer error was lost: %v", export.name, large, err)
			}
		}
	}
}

func TestEmptyGraphCSV(t *testing.T) {
	graph := NewGraph()
	for _, export := range []func(io.Writer) error{graph.NodeToCSV, graph.EdgeToCSV} {
		var out bytes.Buffer
		if err := export(&out); err != nil {
			t.Fatal(err)
		}
		if rows := readCSV(t, out.String()); len(rows) != 1 {
			t.Fatalf("empty graph must export only a header: %v", rows)
		}
	}
}

func TestLoadOSMPreservesWayTags(t *testing.T) {
	tags := Tags{"width": "3;4", "custom": "source value"}
	data := OSMData{Elements: []Element{
		{Type: TypeNode, ID: 1}, {Type: TypeNode, ID: 2},
		{Type: TypeWay, ID: 100, Nodes: []int{1, 2}, Tags: tags},
	}}
	graph := NewGraph()
	if err := graph.LoadOSM(data); err != nil {
		t.Fatal(err)
	}
	tags["width"] = "changed after loading"
	for _, id := range []int{1, 2} {
		edge := graph.adjList[id][0]
		if edge.WayID != 100 || edge.Tags.Get("width") != "3;4" || edge.Tags.Get("custom") != "source value" {
			t.Fatalf("source identity or original tags were lost: %+v", edge)
		}
	}
}
