package qlearning

import (
	"cmp"
	"math"
	"slices"
	"strings"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
)

// Actions indexes access-allowed edges by source node. Candidates are built once;
// travel-dependent masks (revisit, closing, and distance) belong to the environment.
type Actions struct {
	byNode map[int][]*osm.OSMEdge
}

// NewActions filters the graph once by access and positive finite length.
// It stores pointers to the original edges, preserving parallel
// (To, Key) identities without duplicating edge attributes or tags. The graph
// must be non-nil and read-only while these actions are used. Rebuild Actions
// after loading a different graph or changing its edges/access policy.
// Reference: rute_lari_core.py, build_actions (static candidate filtering).
func NewActions(graph *osm.Graph) *Actions {
	actions := &Actions{byNode: make(map[int][]*osm.OSMEdge, len(graph.Nodes))}
	for nodeID := range graph.Nodes {
		var candidates []*osm.OSMEdge
		edges := graph.OutgoingEdges(nodeID)
		for i := range edges {
			// Point to the stored edge, not a copy of a range-loop value.
			edge := &edges[i]
			if edge.LengthM <= 0 || math.IsNaN(edge.LengthM) || math.IsInf(edge.LengthM, 0) {
				continue
			}
			if AccessAllowed(edge.Access) {
				candidates = append(candidates, edge)
			}
		}
		// Sort numerically by destination, then key, for deterministic action
		// order. Only this private pointer slice is reordered; graph edges and
		// their keys remain unchanged. This deliberately differs from the
		// reference's text comparison of keys >=10.
		slices.SortFunc(candidates, func(a, b *osm.OSMEdge) int {
			if order := cmp.Compare(a.To, b.To); order != 0 {
				return order
			}
			return cmp.Compare(a.Key, b.Key)
		})
		if len(candidates) > 0 {
			actions.byNode[nodeID] = candidates
		}
	}
	return actions
}

// AccessAllowed accepts a single access value: empty, yes, or permissive.
// Spaces and case are normalized; all other values are rejected as-is.
// A separate foot tag does not override this research policy.
func AccessAllowed(access string) bool {
	switch strings.ToLower(strings.TrimSpace(access)) {
	case "", "yes", "permissive":
		return true
	default:
		return false
	}
}

// Available returns prebuilt candidates with one map lookup and no filtering or
// result allocation. Unknown, isolated, or blocked nodes return nil. The slice,
// pointed-to edges, and Tags maps are shared read-only data: do not modify,
// append to, or reorder them. The environment must use its own storage for masks.
func (a *Actions) Available(nodeID int) []*osm.OSMEdge {
	return a.byNode[nodeID]
}
