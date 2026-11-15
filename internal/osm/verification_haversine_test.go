package osm

import (
	"math"
	"slices"
	"testing"
)

func TestVerificationWay533112673Haversine(t *testing.T) {
	// Real OSM excerpt: three original nodes and way 533112673, not synthetic data.
	// Production path: Load -> Graph.LoadOSM -> greatCircleDistanceM in geo.go.
	data, err := Load("testdata/way_533112673.json")
	if err != nil {
		t.Fatal(err)
	}
	var way Element
	for _, element := range data.Elements {
		if element.Type == TypeWay && element.ID == 533112673 {
			way = element
		}
	}
	if !slices.Equal(way.Nodes, []int{1720774796, 311432306, 311431486}) {
		t.Fatalf("unexpected nodes for way 533112673: %v", way.Nodes)
	}
	graph := NewGraph()
	if err := graph.LoadOSM(data); err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 3 || graph.EdgeCount() != 4 {
		t.Fatalf("graph has %d nodes and %d edges; want 3 and 4", len(graph.Nodes), graph.EdgeCount())
	}
	t.Logf("Way %d: %s; nodes: %v", way.ID, way.Tags.Get("name"), way.Nodes)
	for _, id := range way.Nodes {
		node := graph.Nodes[id]
		t.Logf("Node %d: latitude=%.7f, longitude=%.7f", id, node.Lat, node.Lon)
	}
	t.Logf("Length source: Graph.LoadOSM -> greatCircleDistanceM (Haversine); R=%d m", earthRadiusM)

	// Independent Haversine reference values, checked to 0.000001 m.
	// Neither the fixture nor this test reads lengths from edges.csv.
	wantEdges := []struct {
		from, to int
		lengthM  float64
	}{
		{1720774796, 311432306, 51.76437838723765},
		{311432306, 1720774796, 51.76437838723765},
		{311432306, 311431486, 161.29387710524682},
		{311431486, 311432306, 161.29387710524682},
	}
	var totalM float64
	for i, want := range wantEdges {
		var found bool
		for _, edge := range graph.OutgoingEdges(want.from) {
			if edge.To != want.to || edge.Key != 0 || edge.WayID != way.ID {
				continue
			}
			if found {
				t.Fatalf("duplicate edge (%d, %d, 0)", want.from, want.to)
			}
			found = true
			from, to := graph.Nodes[edge.From], graph.Nodes[edge.To]
			distanceM, err := GreatCircleDistanceM(from.Lat, from.Lon, to.Lat, to.Lon)
			if err != nil {
				t.Fatal(err)
			}
			if edge.LengthM != distanceM {
				t.Fatalf("edge length %.12f differs from production Haversine %.12f", edge.LengthM, distanceM)
			}
			if math.IsNaN(edge.LengthM) || math.IsInf(edge.LengthM, 0) || math.Abs(edge.LengthM-want.lengthM) > 1e-6 {
				t.Fatalf("edge (%d, %d, 0): length=%.12f m; want %.12f m", edge.From, edge.To, edge.LengthM, want.lengthM)
			}
			t.Logf("Edge (%d, %d, %d): length_m=%.12f m", edge.From, edge.To, edge.Key, edge.LengthM)
			if i == 0 || i == 2 {
				totalM += edge.LengthM
			}
		}
		if !found {
			t.Fatalf("missing edge (%d, %d, 0)", want.from, want.to)
		}
	}
	if math.Abs(totalM-213.05825549248447) > 1e-6 {
		t.Fatalf("forward path length=%.12f m; want 213.05825549248447 m", totalM)
	}
	t.Logf("Path 1720774796 -> 311432306 -> 311431486: %.12f m", totalM)
}
