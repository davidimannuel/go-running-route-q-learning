package qlearning

import (
	"maps"
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
)

func TestAccessAllowed(t *testing.T) {
	for _, tc := range []struct {
		access string
		want   bool
	}{
		{"", true}, {"  ", true}, {"; ;", false},
		{"yes", true}, {"permissive", true}, {" YES ", true},
		{"yes; PERMISSIVE;", false},
		{`["yes", "permissive"]`, false},
		{`['yes', 'permissive']`, false},
		{"[]", false}, {`['']`, false},
		{"permisive", false}, {"private", false}, {"no", false},
		{"destination", false}, {"unknown", false}, {"nan", false},
		{"yes;private", false}, {"permissive;unknown", false},
		{`['yes', 'private']`, false},
		{`["yes", private]`, false}, {`["yes"`, false},
	} {
		t.Run(tc.access, func(t *testing.T) {
			if got := AccessAllowed(tc.access); got != tc.want {
				t.Fatalf("AccessAllowed(%q)=%v; want %v", tc.access, got, tc.want)
			}
		})
	}
}

func actionTestGraph(t testing.TB) *osm.Graph {
	t.Helper()
	graph := osm.NewGraph()
	data := osm.OSMData{Elements: []osm.Element{
		{Type: osm.TypeNode, ID: 1},
		{Type: osm.TypeNode, ID: 2, Lon: 0.001},
		{Type: osm.TypeNode, ID: 3, Lat: 0.001},
		{Type: osm.TypeNode, ID: 4},             // Isolated node.
		{Type: osm.TypeNode, ID: 5, Lon: 0.002}, // Only private roads.
		{Type: osm.TypeWay, ID: 100, Nodes: []int{1, 2}},
		{Type: osm.TypeWay, ID: 101, Nodes: []int{1, 3}, Tags: osm.Tags{"access": "private", "foot": "yes"}},
		{Type: osm.TypeWay, ID: 102, Nodes: []int{1, 2}, Tags: osm.Tags{"access": "yes"}},
		{Type: osm.TypeWay, ID: 103, Nodes: []int{1, 3}, Tags: osm.Tags{"access": "permissive"}},
		{Type: osm.TypeWay, ID: 104, Nodes: []int{1, 1}}, // Zero-length edges.
		{Type: osm.TypeWay, ID: 105, Nodes: []int{5, 3}, Tags: osm.Tags{"access": "private"}},
		{Type: osm.TypeWay, ID: 106, Nodes: []int{1, 3}, Tags: osm.Tags{"access": "yes;private"}},
	}}
	if err := graph.LoadOSM(data); err != nil {
		t.Fatal(err)
	}
	return graph
}

func TestActionsAvailable(t *testing.T) {
	graph := actionTestGraph(t)
	before := make(map[int][]osm.OSMEdge, len(graph.Nodes))
	for nodeID := range graph.Nodes {
		edges := slices.Clone(graph.OutgoingEdges(nodeID))
		for i := range edges {
			edges[i].Tags = maps.Clone(edges[i].Tags)
		}
		before[nodeID] = edges
	}
	actions := NewActions(graph)
	got := actions.Available(1)
	var identities [][3]int
	for _, edge := range got {
		identities = append(identities, [3]int{edge.To, edge.Key, edge.WayID})
	}
	// Distinct keys preserve parallel actions; private/foot=yes and zero-length
	// segments stay in the graph but must not become journey candidates.
	want := [][3]int{{2, 0, 100}, {2, 1, 102}, {3, 1, 103}}
	if !reflect.DeepEqual(identities, want) {
		t.Fatalf("action identities=%v; want %v", identities, want)
	}
	if len(actions.Available(2)) != 2 {
		t.Fatal("legal reverse actions were lost")
	}
	for _, nodeID := range []int{4, 5, 99} {
		if got := actions.Available(nodeID); got != nil {
			t.Fatalf("node %d returned candidates: %v", nodeID, got)
		}
	}
	if len(graph.Nodes) != len(before) {
		t.Fatal("candidate filtering changed the source nodes")
	}
	for nodeID, edges := range before {
		if !reflect.DeepEqual(edges, graph.OutgoingEdges(nodeID)) {
			t.Fatalf("candidate filtering modified source edges at node %d", nodeID)
		}
	}
}

func TestActionsAvailableReusesBuiltCandidates(t *testing.T) {
	graph := actionTestGraph(t)
	actions := NewActions(graph)
	first := actions.Available(1)
	second := actions.Available(1)
	if len(first) != 3 || len(second) != 3 || &first[0] != &second[0] {
		t.Fatal("Available must reuse the prebuilt candidate slice")
	}
	edges := graph.OutgoingEdges(1)
	if first[0] != &edges[0] {
		t.Fatal("candidates must reference graph storage without copying edges")
	}
}

func TestActionsRebuildForDifferentGraph(t *testing.T) {
	graph := actionTestGraph(t)
	if len(NewActions(graph).Available(1)) != 3 {
		t.Fatal("expected three candidates in the original graph")
	}
	if err := graph.LoadOSM(osm.OSMData{}); err != nil {
		t.Fatal(err)
	}
	if got := NewActions(graph).Available(1); got != nil {
		t.Fatalf("rebuilt actions retained the previous graph: %v", got)
	}
}

func TestActionsRejectInvalidLengths(t *testing.T) {
	for _, length := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		graph := osm.NewGraph()
		if err := graph.LoadOSM(osm.OSMData{Elements: []osm.Element{
			{Type: osm.TypeNode, ID: 1}, {Type: osm.TypeNode, ID: 2, Lon: 0.001},
			{Type: osm.TypeWay, ID: 100, Nodes: []int{1, 2}},
		}}); err != nil {
			t.Fatal(err)
		}
		// Inject corrupt data solely to check the action boundary. Production
		// callers must treat OutgoingEdges as read-only.
		graph.OutgoingEdges(1)[0].LengthM = length
		if got := NewActions(graph).Available(1); len(got) != 0 {
			t.Fatalf("invalid length %v became an action", length)
		}
	}
}

func BenchmarkAccessAllowed(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if !AccessAllowed("permissive") {
			b.Fatal("expected access to be allowed")
		}
	}
}

func BenchmarkActionsAvailable(b *testing.B) {
	actions := NewActions(actionTestGraph(b))
	b.ReportAllocs()
	for b.Loop() {
		if len(actions.Available(1)) != 3 {
			b.Fatal("expected three candidates")
		}
	}
}

func BenchmarkNewActions(b *testing.B) {
	graph := actionTestGraph(b)
	b.ReportAllocs()
	for b.Loop() {
		if len(NewActions(graph).Available(1)) != 3 {
			b.Fatal("expected three candidates")
		}
	}
}
