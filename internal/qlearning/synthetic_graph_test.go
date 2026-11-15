package qlearning

import (
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
)

// buildSyntheticGraph returns a fresh copy of the shared seven-node test graph.
// Roads have explicit lengths; geographic coordinates are deliberately absent.
// Direction and geometry cases will need their own explicit distance setup.
func buildSyntheticGraph(t *testing.T) *osm.Graph {
	t.Helper()
	graph := osm.NewGraph()
	roads := []struct {
		from, to int
		lengthM  float64
		tags     osm.Tags
	}{
		{1, 2, 973.4, osm.Tags{"highway": "residential", "surface": "asphalt", "width": "3", "access": "yes"}},
		{2, 3, 1127.6, osm.Tags{"highway": "tertiary", "surface": "concrete", "width": "3"}}, // key 0
		{3, 4, 946.8, osm.Tags{"highway": "residential", "surface": "asphalt", "width": "3", "access": "permissive"}},
		{4, 5, 1083.2, osm.Tags{"highway": "secondary", "width": "2", "access": "yes"}},
		{5, 1, 864.7, osm.Tags{"access": "yes"}}, // Missing road attributes.
		{3, 6, 217.3, osm.Tags{"highway": "service", "surface": "dirt", "width": "2", "access": "yes"}},
		{2, 7, 326.9, osm.Tags{"highway": "residential", "surface": "asphalt", "width": "3", "access": "yes"}},
		{2, 4, 548.6, osm.Tags{"highway": "footway", "surface": "asphalt", "width": "3", "access": "private"}},
		{2, 3, 1189.5, osm.Tags{"highway": "residential", "surface": "gravel", "access": "yes"}}, // key 1
	}
	for _, road := range roads {
		if err := graph.AddRoad(road.from, road.to, road.lengthM, road.tags); err != nil {
			t.Fatal(err)
		}
	}
	// The way is allowed, but its endpoint is private.
	node := graph.Nodes[7]
	node.Tags = osm.Tags{"access": "private"}
	graph.Nodes[7] = node
	if len(graph.Nodes) != 7 || graph.EdgeCount() != 18 {
		t.Fatal("expected seven nodes and nine bidirectional roads")
	}
	return graph
}
