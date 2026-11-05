package osm

import "fmt"

// Graph stores a directed multigraph as a list of outgoing edges for each node.
// Parallel edges are retained so each road segment remains a separate choice.
// References:
//   - Adjacency lists: https://www.geeksforgeeks.org/dsa/graph-and-its-representations/
//   - Directed multigraphs: https://networkx.org/documentation/stable/reference/classes/multidigraph.html
type Graph struct {
	// Nodes preserves OSM node IDs and coordinates, including isolated nodes.
	Nodes map[int]Element
	// adjList[u] contains only outgoing edges whose From equals u.
	adjList map[int][]OSMEdge
}

// NewGraph returns an empty graph with initialized node and adjacency maps.
func NewGraph() *Graph {
	return &Graph{
		Nodes:   make(map[int]Element),
		adjList: make(map[int][]OSMEdge),
	}
}

// OSMEdge describes one directed road segment and its OSM attributes.
// Parallel segments have separate keys even when their endpoints are identical.
type OSMEdge struct {
	From    int
	To      int
	Key     int
	LengthM float64
	Name    string
	Highway string
	Surface string
	Width   float64
	Access  string
}

// LoadOSM replaces the graph with bidirectional segments between consecutive way nodes.
// LengthM remains zero until distance calculation is implemented.
// Invalid input leaves the previous graph unchanged; repeated loads replace old data.
// Every segment currently gets a reverse edge; walking direction rules are a later step.
// OSM ways contain ordered node references: https://wiki.openstreetmap.org/wiki/Way
func (g *Graph) LoadOSM(data OSMData) error {
	// Build separately so an error cannot leave the receiver partially updated.
	graph := NewGraph()

	// Nodes must exist before any way is processed, regardless of element order.
	for _, element := range data.Elements {
		if element.Type == TypeNode {
			graph.Nodes[element.ID] = element
			// A nil slice keeps isolated nodes visible without allocating an empty backing array.
			graph.adjList[element.ID] = nil
		}
	}

	for _, way := range data.Elements {
		if way.Type != TypeWay {
			continue
		}
		if len(way.Nodes) < 2 {
			return fmt.Errorf("load OSM graph: way %d must contain at least two node references", way.ID)
		}
		// Reject incomplete ways rather than creating edges with missing endpoints.
		for _, nodeID := range way.Nodes {
			if _, ok := graph.Nodes[nodeID]; !ok {
				return fmt.Errorf("load OSM graph: way %d references missing node %d", way.ID, nodeID)
			}
		}

		// All segments from this way inherit its tags; parse numeric width once.
		edge := OSMEdge{
			Name:    way.Tags.Get("name"),
			Highway: way.Tags.Get("highway"),
			Surface: way.Tags.Get("surface"),
			Width:   way.Tags.GetFloat64("width"),
			Access:  way.Tags.Get("access"),
		}
		// [A, B, C] becomes A->B and B->C, each with its reverse direction.
		for i := 1; i < len(way.Nodes); i++ {
			edge.From, edge.To = way.Nodes[i-1], way.Nodes[i]
			graph.addEdge(edge)

			// addEdge stores a value copy, so reversing this value preserves the forward edge.
			edge.From, edge.To = edge.To, edge.From
			graph.addEdge(edge)
		}
	}

	g.Nodes = graph.Nodes
	g.adjList = graph.adjList
	return nil
}

func (g *Graph) addEdge(edge OSMEdge) {
	// Keys use the outgoing index per source node in this implementation.
	// They are distinct from OSM way IDs and remain unique while edges are only appended.
	edge.Key = len(g.adjList[edge.From])
	g.adjList[edge.From] = append(g.adjList[edge.From], edge)
}

// PrintInfo prints outgoing edges for manual inspection during development.
func (g *Graph) PrintInfo(nodeID int) {
	fmt.Printf("nodeID %d\n", nodeID)
	for _, edge := range g.adjList[nodeID] {
		fmt.Printf("\t%d->%d, %d, %s, %s, %s, %.8f\n",
			edge.From, edge.To, edge.Key, edge.Name, edge.Highway, edge.Surface, edge.Width)
	}
}
