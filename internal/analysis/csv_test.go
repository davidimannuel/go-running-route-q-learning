package analysis

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

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
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
	graph := osm.NewGraph()
	graph.Nodes[30] = osm.Element{ID: 30}
	graph.Nodes[9620836306] = osm.Element{ID: 9620836306, Lat: -7.282187123456789, Lon: 112.70928212345678}
	graph.Nodes[10] = osm.Element{ID: 10, Lat: -7.25, Lon: 112.5}
	var out bytes.Buffer
	if err := NodeToCSV(&out, graph); err != nil {
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
	graph := osm.NewGraph()
	if err := graph.LoadOSM(osm.OSMData{Elements: []osm.Element{
		{Type: osm.TypeNode, ID: 1}, {Type: osm.TypeNode, ID: 2, Lon: 0.001}, {Type: osm.TypeNode, ID: 3, Lon: 0.002},
		{Type: osm.TypeWay, ID: 8, Nodes: []int{1, 3}, Tags: osm.Tags{
			"name": "Road, \"A\"\nNorth", "highway": "residential", "surface": "asphalt", "width": "3;4", "access": "permissive",
		}},
		{Type: osm.TypeWay, ID: 9, Nodes: []int{1, 2}},
		{Type: osm.TypeWay, ID: 10, Nodes: []int{1, 2}, Tags: osm.Tags{"width": "3 m"}},
	}}); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"from", "to", "key", "way_id", "length_m", "name", "highway", "surface", "width", "access"},
		{"1", "2", "0", "9", "111.19508372419142", "", "", "", "", ""},
		{"1", "2", "1", "10", "111.19508372419142", "", "", "", "3 m", ""},
		{"1", "3", "0", "8", "222.39016744838284", "Road, \"A\"\nNorth", "residential", "asphalt", "3;4", "permissive"},
		{"2", "1", "0", "9", "111.19508372419142", "", "", "", "", ""},
		{"2", "1", "1", "10", "111.19508372419142", "", "", "", "3 m", ""},
		{"3", "1", "0", "8", "222.39016744838284", "Road, \"A\"\nNorth", "residential", "asphalt", "3;4", "permissive"},
	}
	for range 2 {
		var out bytes.Buffer
		if err := EdgeToCSV(&out, graph); err != nil {
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
		for i, scores := range [][4]float64{
			{-0.5, -0.5, -0.5, -0.5}, {-0.5, -0.5, 1, -0.275}, {1, 1, -0.5, 0.775},
			{-0.5, -0.5, -0.5, -0.5}, {-0.5, -0.5, 1, -0.275}, {1, 1, -0.5, 0.775},
		} {
			for j, wantScore := range scores {
				value, err := strconv.ParseFloat(got[i+1][10+j], 64)
				if err != nil || math.Abs(value-wantScore) > 1e-12 {
					t.Fatalf("score=%v; want %v", got[i+1], scores)
				}
			}
		}
	}
	edges := graph.OutgoingEdges(1)
	if edges[0].To != 3 || edges[1].Key != 0 || edges[2].Key != 1 {
		t.Fatal("CSV sorting changed adjacency order or action keys")
	}
}

type failingCSVWriter struct{ err error }

func (w failingCSVWriter) Write([]byte) (int, error) { return 0, w.err }

func TestCSVWriteErrors(t *testing.T) {
	for _, large := range []bool{false, true} {
		graph := osm.NewGraph()
		if large {
			var data osm.OSMData
			for id := range 500 {
				data.Elements = append(data.Elements, osm.Element{Type: osm.TypeNode, ID: id, Lat: 1, Lon: 2})
			}
			data.Elements = append(data.Elements, osm.Element{Type: osm.TypeWay, ID: 100, Nodes: []int{1, 2}, Tags: osm.Tags{"name": strings.Repeat("x", 5000)}})
			if err := graph.LoadOSM(data); err != nil {
				t.Fatal(err)
			}
		}
		for _, export := range []struct {
			name  string
			write func(io.Writer) error
		}{
			{name: "nodes", write: func(w io.Writer) error { return NodeToCSV(w, graph) }},
			{name: "edges", write: func(w io.Writer) error { return EdgeToCSV(w, graph) }},
		} {
			sentinel := errors.New("writer unavailable")
			if err := export.write(failingCSVWriter{err: sentinel}); !errors.Is(err, sentinel) {
				t.Fatalf("%s (large=%v): writer error was lost: %v", export.name, large, err)
			}
		}
	}
}

func TestEmptyGraphCSV(t *testing.T) {
	graph := osm.NewGraph()
	for _, export := range []func(io.Writer) error{
		func(w io.Writer) error { return NodeToCSV(w, graph) },
		func(w io.Writer) error { return EdgeToCSV(w, graph) },
	} {
		var out bytes.Buffer
		if err := export(&out); err != nil {
			t.Fatal(err)
		}
		if rows := readCSV(t, out.String()); len(rows) != 1 {
			t.Fatalf("empty graph must export only a header: %v", rows)
		}
	}
}
