package osm

import (
	"fmt"
	"maps"
)

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
// Key starts at zero for each ordered (From, To) pair and distinguishes parallel edges.
// Different destinations and the reverse direction have independent key sequences.
type OSMEdge struct {
	From int
	To   int
	Key  int
	// WayID links each raw segment back to its source OSM way.
	WayID int
	// LengthM is the great-circle distance of this raw segment, in meters.
	LengthM float64
	Name    string
	Highway string
	Surface string
	// Width is the parsed width in meters. WidthValid is false for missing,
	// invalid, or combined tags; Width is then zero. Original text stays in Tags.
	Width      float64
	WidthValid bool
	Access     string
	// Tags preserves original values, including missing and non-numeric width tags.
	// Edges from the same way share this cloned map; treat it as read-only.
	Tags Tags
}

// LoadOSM replaces the graph with bidirectional segments between consecutive way nodes.
// LengthM is computed per raw segment from node coordinates, before simplification.
// Reference: https://github.com/gboeing/osmnx/blob/v2.1.1/osmnx/distance.py#L158-L179
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

		// Parse once per way, then reuse the value and validity for every segment
		// and its reverse. CSV and map exports retain the original width text.
		widthM, widthValid := ParseWidthM(way.Tags.Get("width"))
		edge := OSMEdge{
			WayID:      way.ID,
			Tags:       maps.Clone(way.Tags),
			Name:       way.Tags.Get("name"),
			Highway:    way.Tags.Get("highway"),
			Surface:    way.Tags.Get("surface"),
			Width:      widthM,
			WidthValid: widthValid,
			Access:     way.Tags.Get("access"),
		}
		// [A, B, C] becomes A->B and B->C, each with its reverse direction.
		for i := 1; i < len(way.Nodes); i++ {
			edge.From, edge.To = way.Nodes[i-1], way.Nodes[i]
			from, to := graph.Nodes[edge.From], graph.Nodes[edge.To]
			lengthM, err := greatCircleDistanceM(from.Lat, from.Lon, to.Lat, to.Lon)
			if err != nil {
				return fmt.Errorf("load OSM graph: way %d segment %d->%d: %w", way.ID, edge.From, edge.To, err)
			}
			// The reverse edge reuses this length; both directions follow the same segment.
			edge.LengthM = lengthM
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
	// Edges are only appended, so counting existing edges to this destination
	// gives the next key for the ordered pair. Other destinations do not affect it.
	edge.Key = 0
	for _, existing := range g.adjList[edge.From] {
		if existing.To == edge.To {
			edge.Key++
		}
	}
	g.adjList[edge.From] = append(g.adjList[edge.From], edge)
}

// PrintInfo prints outgoing edges for manual inspection during development.
func (g *Graph) PrintInfo(nodeID int) {
	fmt.Printf("nodeID %d\n", nodeID)
	for _, edge := range g.adjList[nodeID] {
		fmt.Printf("\t%d->%d, %d, %s, %s, %s, %.8f, length_m=%.6f\n",
			edge.From, edge.To, edge.Key, edge.Name, edge.Highway, edge.Surface, edge.Width, edge.LengthM)
	}
}

// EdgeCount counts directed edges, including reverse and parallel segments.
func (g *Graph) EdgeCount() int {
	count := 0
	for _, edges := range g.adjList {
		count += len(edges)
	}
	return count
}
