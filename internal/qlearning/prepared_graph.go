package qlearning

import (
	"fmt"
	"math"
	"strings"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
)

type edgeData struct {
	comfort, accessReward float64
	roadID                int
}

// PreparedGraph holds graph-derived data prepared for a fixed start node.
// It is shared read-only by independent seed workers.
// Only the environment and Q-table are reset/mutated during training.
type PreparedGraph struct {
	graph     *osm.Graph
	reward    RewardConfig
	startNode int
	actions   *Actions
	distances map[int]float64 // Straight-line distance from each node to startNode, in meters (Haversine).
	edges     map[*osm.OSMEdge]edgeData
}

// NearestStart considers all graph nodes, like the reference nearest-node snap.
// Ties use the smaller ID, avoiding randomized Go map iteration. A nearest node
// with disallowed access or no legal outgoing edges is rejected rather than silently
// selecting a different node.
func NearestStart(graph *osm.Graph, lat, lon float64) (int, float64, error) {
	if graph == nil || len(graph.Nodes) == 0 {
		return 0, 0, fmt.Errorf("nearest start: graph has no nodes")
	}
	id, nearest := 0, math.Inf(1)
	for nodeID, node := range graph.Nodes {
		distance, err := osm.GreatCircleDistanceM(lat, lon, node.Lat, node.Lon)
		if err != nil {
			return 0, 0, fmt.Errorf("nearest start: node %d: %w", nodeID, err)
		}
		if distance < nearest || (distance == nearest && nodeID < id) {
			id, nearest = nodeID, distance
		}
	}
	return id, nearest, nil
}

// NewPreparedGraph precomputes start distances, comfort/access rewards, and physical
// road IDs once. The graph must remain read-only while the prepared graph is used.
func NewPreparedGraph(graph *osm.Graph, startNode int, reward RewardConfig) (*PreparedGraph, error) {
	if err := reward.Validate(); err != nil {
		return nil, err
	}
	if graph == nil {
		return nil, fmt.Errorf("prepare training: graph is nil")
	}
	start, ok := graph.Nodes[startNode]
	if !ok {
		return nil, fmt.Errorf("prepare training: start node %d is missing", startNode)
	}
	if access := start.Tags.Get("access"); !AccessAllowed(access) {
		return nil, fmt.Errorf("prepare training: start node %d has disallowed access %q", startNode, access)
	}
	p := &PreparedGraph{graph: graph, reward: reward, startNode: startNode, actions: NewActions(graph),
		distances: make(map[int]float64, len(graph.Nodes)), edges: make(map[*osm.OSMEdge]edgeData)}
	for id, node := range graph.Nodes {
		distance, err := osm.GreatCircleDistanceM(node.Lat, node.Lon, start.Lat, start.Lon)
		if err != nil {
			return nil, fmt.Errorf("prepare training: node %d: %w", id, err)
		}
		p.distances[id] = distance
	}
	// Assign road IDs to track overlap. Edges with the same geometry share an ID,
	// including reverse directions. Like the reference fallback, our raw OSM
	// segments use endpoint coordinates; polylines would need the full geometry.
	roads := make(map[[4]float64]int)
	for _, candidates := range p.actions.byNode {
		for _, edge := range candidates {
			from, fromOK := graph.Nodes[edge.From]
			to, toOK := graph.Nodes[edge.To]
			if !fromOK || !toOK {
				return nil, fmt.Errorf("prepare training: edge references missing node")
			}
			a, b := [2]float64{from.Lat, from.Lon}, [2]float64{to.Lat, to.Lon}
			if a[0] > b[0] || (a[0] == b[0] && a[1] > b[1]) {
				a, b = b, a
			}
			geometry := [4]float64{a[0], a[1], b[0], b[1]}
			roadID, exists := roads[geometry]
			if !exists {
				roadID = len(roads)
				roads[geometry] = roadID
			}
			data := edgeData{comfort: reward.Road.ScoreEdge(*edge).ComfortScore, roadID: roadID}
			if strings.EqualFold(strings.TrimSpace(edge.Access), "permissive") {
				data.accessReward = reward.PermissivePenaltyPer100M * edge.LengthM / 100
			}
			p.edges[edge] = data
		}
	}
	return p, nil
}
func (p *PreparedGraph) StartNode() int        { return p.startNode }
func (p *PreparedGraph) Graph() *osm.Graph     { return p.graph }
func (p *PreparedGraph) HasStartActions() bool { return len(p.actions.Available(p.startNode)) > 0 }
