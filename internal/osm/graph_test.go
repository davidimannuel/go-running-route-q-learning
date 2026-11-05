package osm

import (
	"maps"
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
			want := map[int][]OSMEdge{
				1: {
					{From: 1, To: 2, Key: 0, Name: "Main Street", Highway: "residential", Surface: "asphalt", Width: 4, Access: "permissive"},
					{From: 1, To: 2, Key: 1},
				},
				2: {
					{From: 2, To: 1, Key: 0, Name: "Main Street", Highway: "residential", Surface: "asphalt", Width: 4, Access: "permissive"},
					{From: 2, To: 3, Key: 1, Name: "Main Street", Highway: "residential", Surface: "asphalt", Width: 4, Access: "permissive"},
					{From: 2, To: 1, Key: 2},
				},
				3: {{From: 3, To: 2, Key: 0, Name: "Main Street", Highway: "residential", Surface: "asphalt", Width: 4, Access: "permissive"}},
				4: nil,
			}
			if !reflect.DeepEqual(graph.adjList, want) {
				t.Fatalf("adjacency list=%+v; want %+v", graph.adjList, want)
			}
		})
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
