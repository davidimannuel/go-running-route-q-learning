package osm

import (
	"cmp"
	"encoding/csv"
	"fmt"
	"io"
	"slices"
	"strconv"
)

// NodeToCSV writes a header and one row per node, ordered by node ID.
// The caller owns the writer; this method flushes CSV buffering and returns both
// immediate and buffered write failures. Coordinates retain float64 precision.
// Reference: https://pkg.go.dev/encoding/csv#Writer
func (g *Graph) NodeToCSV(w io.Writer) error {
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"node_id", "latitude", "longitude"}); err != nil {
		return fmt.Errorf("export nodes CSV: write header: %w", err)
	}
	ids := make([]int, 0, len(g.Nodes))
	for nodeID := range g.Nodes {
		ids = append(ids, nodeID)
	}
	slices.Sort(ids)
	for _, nodeID := range ids {
		node := g.Nodes[nodeID]
		if err := writer.Write([]string{
			strconv.Itoa(nodeID),
			strconv.FormatFloat(node.Lat, 'f', -1, 64),
			strconv.FormatFloat(node.Lon, 'f', -1, 64),
		}); err != nil {
			return fmt.Errorf("export nodes CSV: write node %d: %w", nodeID, err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("export nodes CSV: flush: %w", err)
	}
	return nil
}

// EdgeToCSV writes directed edges ordered by (from, to, key), retaining reverse
// and parallel edges. Width is the original tag, so missing/invalid data remains
// distinguishable from zero. The exported road attributes are highway, surface,
// width, and access, matching the inputs used by the route model.
// Scores use the same Go mapping as the map export.
// Sorting a copy preserves adjacency order and the existing action keys.
func (g *Graph) EdgeToCSV(w io.Writer) error {
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{
		"from", "to", "key", "way_id", "length_m", "name", "highway", "surface", "width", "access",
		"highway_score", "surface_score", "width_score", "comfort_score",
	}); err != nil {
		return fmt.Errorf("export edges CSV: write header: %w", err)
	}
	edges := g.sortedEdges()
	for _, edge := range edges {
		scores := edge.Scores()
		if err := writer.Write([]string{
			strconv.Itoa(edge.From), strconv.Itoa(edge.To), strconv.Itoa(edge.Key),
			strconv.Itoa(edge.WayID), strconv.FormatFloat(edge.LengthM, 'f', -1, 64),
			edge.Name, edge.Highway, edge.Surface, edge.Tags.Get("width"),
			edge.Access,
			strconv.FormatFloat(scores.HighwayScore, 'f', -1, 64),
			strconv.FormatFloat(scores.SurfaceScore, 'f', -1, 64),
			strconv.FormatFloat(scores.WidthScore, 'f', -1, 64),
			strconv.FormatFloat(scores.ComfortScore, 'f', -1, 64),
		}); err != nil {
			return fmt.Errorf("export edges CSV: write edge (%d, %d, %d): %w", edge.From, edge.To, edge.Key, err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("export edges CSV: flush: %w", err)
	}
	return nil
}

// sortedEdges returns a deterministic copy without reordering action adjacency.
func (g *Graph) sortedEdges() []OSMEdge {
	edges := make([]OSMEdge, 0, g.EdgeCount())
	for _, outgoing := range g.adjList {
		edges = append(edges, outgoing...)
	}
	slices.SortFunc(edges, func(a, b OSMEdge) int {
		if order := cmp.Compare(a.From, b.From); order != 0 {
			return order
		}
		if order := cmp.Compare(a.To, b.To); order != 0 {
			return order
		}
		return cmp.Compare(a.Key, b.Key)
	})
	return edges
}
