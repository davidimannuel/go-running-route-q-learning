package osm

import (
	"maps"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func graphTestData(waysFirst bool) OSMData {
	nodes := []Element{
		{Type: TypeNode, ID: 1},
		{Type: TypeNode, ID: 2},
		{Type: TypeNode, ID: 3},
		{Type: TypeNode, ID: 4}, // An isolated node must still be retained.
	}
	ways := []Element{
		{Type: TypeWay, ID: 100, Nodes: []int{1, 2, 3}, Tags: Tags{
			"name": "Main Street", "highway": "residential", "surface": "asphalt",
			"width": "4", "access": "permissive",
		}},
		{Type: TypeWay, ID: 101, Nodes: []int{1, 2}}, // A parallel segment without tags.
	}
	if waysFirst {
		return OSMData{Elements: append(ways, nodes...)}
	}
	return OSMData{Elements: append(nodes, ways...)}
}

func TestLoadOSM(t *testing.T) {
	for _, waysFirst := range []bool{false, true} {
		name := "nodes before ways"
		if waysFirst {
			name = "ways before nodes"
		}
		t.Run(name, func(t *testing.T) {
			graph := NewGraph()
			if err := graph.LoadOSM(graphTestData(waysFirst)); err != nil {
				t.Fatal(err)
			}
			if len(graph.Nodes) != 4 {
				t.Fatalf("nodes=%d; want 4", len(graph.Nodes))
			}
			tags := Tags{"name": "Main Street", "highway": "residential", "surface": "asphalt", "width": "4", "access": "permissive"}
			want := map[int][]OSMEdge{
				1: {
					{From: 1, To: 2, Key: 0, WayID: 100, Tags: tags, Name: "Main Street", Highway: "residential", Surface: "asphalt", Width: 4, WidthValid: true, Access: "permissive"},
					{From: 1, To: 2, Key: 1, WayID: 101},
				},
				2: {
					{From: 2, To: 1, Key: 0, WayID: 100, Tags: tags, Name: "Main Street", Highway: "residential", Surface: "asphalt", Width: 4, WidthValid: true, Access: "permissive"},
					{From: 2, To: 3, Key: 0, WayID: 100, Tags: tags, Name: "Main Street", Highway: "residential", Surface: "asphalt", Width: 4, WidthValid: true, Access: "permissive"},
					{From: 2, To: 1, Key: 1, WayID: 101},
				},
				3: {{From: 3, To: 2, Key: 0, WayID: 100, Tags: tags, Name: "Main Street", Highway: "residential", Surface: "asphalt", Width: 4, WidthValid: true, Access: "permissive"}},
				4: nil,
			}
			if !reflect.DeepEqual(graph.adjList, want) {
				t.Fatalf("adjacency list=%+v; want %+v", graph.adjList, want)
			}
		})
	}
}

// Interleaved destinations must not consume keys from another ordered pair.
func TestLoadOSMKeysPerDirectedPair(t *testing.T) {
	graph := NewGraph()
	data := OSMData{Elements: []Element{
		{Type: TypeNode, ID: 1}, {Type: TypeNode, ID: 2}, {Type: TypeNode, ID: 3},
		{Type: TypeWay, ID: 100, Nodes: []int{1, 2}},
		{Type: TypeWay, ID: 101, Nodes: []int{1, 3}},
		{Type: TypeWay, ID: 102, Nodes: []int{1, 2}},
		{Type: TypeWay, ID: 103, Nodes: []int{2, 3}},
		{Type: TypeWay, ID: 104, Nodes: []int{1, 2}},
	}}
	if err := graph.LoadOSM(data); err != nil {
		t.Fatal(err)
	}
	want := map[[2]int][]int{
		{1, 2}: {0, 1, 2}, {2, 1}: {0, 1, 2},
		{1, 3}: {0}, {3, 1}: {0},
		{2, 3}: {0}, {3, 2}: {0},
	}
	got := make(map[[2]int][]int)
	for _, edges := range graph.adjList {
		for _, edge := range edges {
			pair := [2]int{edge.From, edge.To}
			got[pair] = append(got[pair], edge.Key)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("keys per directed pair=%v; want %v", got, want)
	}
}

func TestLoadOSMReplacesGraph(t *testing.T) {
	// Loading also works on a graph whose maps have not been initialized.
	var graph Graph
	data := graphTestData(false)
	if err := graph.LoadOSM(data); err != nil {
		t.Fatal(err)
	}
	previous := graph.adjList
	if err := graph.LoadOSM(data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(graph.adjList, previous) {
		t.Fatal("loading the same data duplicated or changed the edges")
	}
	if err := graph.LoadOSM(OSMData{Elements: []Element{{Type: TypeNode, ID: 99}}}); err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 1 || graph.Nodes[99].ID != 99 || len(graph.adjList) != 1 {
		t.Fatalf("previous graph was not replaced: nodes=%v, edges=%v", graph.Nodes, graph.adjList)
	}
	if err := graph.LoadOSM(OSMData{}); err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 0 || len(graph.adjList) != 0 {
		t.Fatal("loading empty data must clear the graph")
	}
}

func TestLoadOSMRejectsInvalidWays(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		nodes      []int
	}{
		{name: "empty way", want: "way 200 must contain at least two node references"},
		{name: "one node", nodes: []int{1}, want: "way 200 must contain at least two node references"},
		{name: "missing source", nodes: []int{99, 1}, want: "way 200 references missing node 99"},
		{name: "missing destination", nodes: []int{1, 99}, want: "way 200 references missing node 99"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph := NewGraph()
			if err := graph.LoadOSM(graphTestData(false)); err != nil {
				t.Fatal(err)
			}
			previousNodes := maps.Clone(graph.Nodes)
			previousEdges := make(map[int][]OSMEdge, len(graph.adjList))
			for nodeID, edges := range graph.adjList {
				previousEdges[nodeID] = slices.Clone(edges)
			}
			data := graphTestData(true)
			data.Elements = append(data.Elements, Element{Type: TypeWay, ID: 200, Nodes: tc.nodes})
			err := graph.LoadOSM(data)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v; want %q", err, tc.want)
			}
			if !reflect.DeepEqual(graph.Nodes, previousNodes) || !reflect.DeepEqual(graph.adjList, previousEdges) {
				t.Fatal("invalid data modified the previous graph")
			}
		})
	}
}

func TestLoadOSMCalculatesSegmentLengths(t *testing.T) {
	data := OSMData{Elements: []Element{
		{Type: TypeNode, ID: 1, Lat: 0, Lon: 0},
		{Type: TypeNode, ID: 2, Lat: 0, Lon: 0.001},
		{Type: TypeNode, ID: 3, Lat: 0.002, Lon: 0.001},
		{Type: TypeWay, ID: 100, Nodes: []int{1, 2, 3}},
		{Type: TypeWay, ID: 101, Nodes: []int{1, 2}},
	}}
	graph := NewGraph()
	if err := graph.LoadOSM(data); err != nil {
		t.Fatal(err)
	}
	// One eastward arc and one northward arc have different known lengths.
	// Check every directed edge, including the parallel segment and both reverses.
	wantM := map[[3]int]float64{
		{1, 2, 0}: 111.19508372419142,
		{2, 1, 0}: 111.19508372419142,
		{2, 3, 0}: 222.39016744838284,
		{3, 2, 0}: 222.39016744838284,
		{1, 2, 1}: 111.19508372419142,
		{2, 1, 1}: 111.19508372419142,
	}
	for _, edges := range graph.adjList {
		for _, edge := range edges {
			id := [3]int{edge.From, edge.To, edge.Key}
			want, ok := wantM[id]
			if !ok || math.IsNaN(edge.LengthM) || math.IsInf(edge.LengthM, 0) || math.Abs(edge.LengthM-want) > 1e-6 {
				t.Fatalf("edge %v: length=%.12f m; want %.12f m (expected edge=%v)", id, edge.LengthM, want, ok)
			}
			delete(wantM, id)
		}
	}
	if len(wantM) != 0 {
		t.Fatalf("missing expected edges: %v", wantM)
	}
	if graph.adjList[1][0].LengthM != graph.adjList[2][0].LengthM {
		t.Fatal("forward and reverse edges must store exactly the same length")
	}
	// Accumulate the segments along 1->2->3, rather than measuring endpoint 1->3.
	totalM := graph.adjList[1][0].LengthM + graph.adjList[2][1].LengthM
	if math.Abs(totalM-333.58525117257426) > 1e-6 {
		t.Fatalf("path distance=%.12f m; want 333.58525117257426 m", totalM)
	}
}

func TestLoadOSMRejectsInvalidSegmentCoordinates(t *testing.T) {
	for _, tc := range []struct {
		name     string
		lat, lon float64
	}{
		{name: "latitude out of range", lat: 91},
		{name: "longitude out of range", lon: 181},
		{name: "NaN latitude", lat: math.NaN()},
		{name: "infinite longitude", lon: math.Inf(1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph := NewGraph()
			if err := graph.LoadOSM(OSMData{Elements: []Element{{Type: TypeNode, ID: 99}}}); err != nil {
				t.Fatal(err)
			}
			data := OSMData{Elements: []Element{
				{Type: TypeNode, ID: 1},
				{Type: TypeNode, ID: 2, Lat: 0, Lon: 0.001},
				{Type: TypeNode, ID: 3, Lat: tc.lat, Lon: tc.lon},
				{Type: TypeWay, ID: 100, Nodes: []int{1, 2, 3}},
			}}
			err := graph.LoadOSM(data)
			if err == nil || !strings.Contains(err.Error(), "way 100 segment 2->3") {
				t.Fatalf("expected an error identifying the failed way and segment; got %v", err)
			}
			if len(graph.Nodes) != 1 || graph.Nodes[99].ID != 99 || len(graph.adjList) != 1 || len(graph.adjList[99]) != 0 {
				t.Fatalf("failed load modified the previous graph: nodes=%v, edges=%v", graph.Nodes, graph.adjList)
			}
		})
	}
}

func TestLoadOSMStoresParsedWidth(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		meters    float64
		valid     bool
	}{
		{"plain", "2", 2, true},
		{"meter unit", "3 m", 3, true},
		{"decimal comma", "3,5 metres", 3.5, true},
		{"missing", "", 0, false},
		{"combined", "3;4", 0, false},
		{"invalid", "unknown", 0, false},
		{"zero", "0", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph := NewGraph()
			data := OSMData{Elements: []Element{
				{Type: TypeNode, ID: 1}, {Type: TypeNode, ID: 2}, {Type: TypeNode, ID: 3},
				{Type: TypeWay, ID: 100, Nodes: []int{1, 2, 3}, Tags: Tags{"width": tc.raw}},
			}}
			if err := graph.LoadOSM(data); err != nil {
				t.Fatal(err)
			}
			if graph.EdgeCount() != 4 {
				t.Fatal("expected two segments and their reverses")
			}
			for _, edge := range graph.sortedEdges() {
				if edge.Width != tc.meters || edge.WidthValid != tc.valid || edge.Tags.Get("width") != tc.raw {
					t.Fatalf("width=%v, valid=%v, raw=%q; want %v, %v, %q", edge.Width, edge.WidthValid, edge.Tags.Get("width"), tc.meters, tc.valid, tc.raw)
				}
			}
		})
	}
}
