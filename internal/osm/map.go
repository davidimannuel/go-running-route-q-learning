package osm

import (
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"math"
	"slices"
	"strconv"
)

//go:embed road-analysis.gohtml
var roadAnalysisHTML string

var roadAnalysisTemplate = template.Must(template.New("road-analysis").Parse(roadAnalysisHTML))

type mapNode struct {
	ID        string  `json:"id"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type mapEdge struct {
	From    string  `json:"from"`
	To      string  `json:"to"`
	Key     int     `json:"key"`
	WayID   string  `json:"way_id"`
	LengthM float64 `json:"length_m"`
	Name    string  `json:"name"`
	Highway string  `json:"highway"`
	Surface string  `json:"surface"`
	Width   string  `json:"width"`
	Access  string  `json:"access"`
	RoadScores
}

// A display segment groups coincident raw edges only. All directed identities,
// parallel actions, attributes, lengths, and scores remain in the edge table.
type mapSegment struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Edges []int  `json:"edges"`
}

type mapData struct {
	Center   [2]float64   `json:"center"`
	Nodes    []mapNode    `json:"nodes"`
	Edges    []mapEdge    `json:"edges"`
	Segments []mapSegment `json:"segments"`
	Weights  [3]float64   `json:"weights"`
}

// MapToHTML embeds the current raw graph, scores, and configured input location
// in one HTML file. No local JSON fetch, graph mutation, nearest-start selection,
// network request, or training is performed. Leaflet/OSM assets load in-browser.
// IDs are strings because JavaScript numbers cannot represent all Go integers.
func (g *Graph) MapToHTML(w io.Writer, latitude, longitude float64) error {
	if _, err := greatCircleDistanceM(latitude, longitude, latitude, longitude); err != nil {
		return fmt.Errorf("export map: input location: %w", err)
	}
	data := mapData{
		Center:   [2]float64{latitude, longitude},
		Nodes:    make([]mapNode, 0, len(g.Nodes)),
		Edges:    make([]mapEdge, 0, g.EdgeCount()),
		Segments: make([]mapSegment, 0),
		Weights:  [3]float64{HighwayWeight, SurfaceWeight, WidthWeight},
	}
	ids := make([]int, 0, len(g.Nodes))
	for id := range g.Nodes {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		node := g.Nodes[id]
		if _, err := greatCircleDistanceM(node.Lat, node.Lon, node.Lat, node.Lon); err != nil {
			return fmt.Errorf("export map: node %d: %w", id, err)
		}
		data.Nodes = append(data.Nodes, mapNode{ID: strconv.Itoa(id), Latitude: node.Lat, Longitude: node.Lon})
	}
	segmentIndex := make(map[[2]int]int)
	for _, edge := range g.sortedEdges() {
		if _, ok := g.Nodes[edge.From]; !ok {
			return fmt.Errorf("export map: edge references missing source node %d", edge.From)
		}
		if _, ok := g.Nodes[edge.To]; !ok {
			return fmt.Errorf("export map: edge references missing destination node %d", edge.To)
		}
		if math.IsNaN(edge.LengthM) || math.IsInf(edge.LengthM, 0) || edge.LengthM < 0 {
			return fmt.Errorf("export map: invalid edge length for (%d, %d, %d)", edge.From, edge.To, edge.Key)
		}
		index := len(data.Edges)
		data.Edges = append(data.Edges, mapEdge{
			From: strconv.Itoa(edge.From), To: strconv.Itoa(edge.To), Key: edge.Key,
			WayID: strconv.Itoa(edge.WayID), LengthM: edge.LengthM,
			Name: edge.Name, Highway: edge.Highway, Surface: edge.Surface,
			Width: edge.Tags.Get("width"), Access: edge.Access, RoadScores: edge.Scores(),
		})
		pair := [2]int{min(edge.From, edge.To), max(edge.From, edge.To)}
		position, exists := segmentIndex[pair]
		if !exists {
			position = len(data.Segments)
			segmentIndex[pair] = position
			data.Segments = append(data.Segments, mapSegment{
				From: strconv.Itoa(pair[0]), To: strconv.Itoa(pair[1]), Edges: make([]int, 0, 2),
			})
		}
		data.Segments[position].Edges = append(data.Segments[position].Edges, index)
	}
	// html/template serializes this typed value as JSON in script context and
	// escapes <, >, and &. Original OSM strings cannot terminate the data script.
	if err := roadAnalysisTemplate.Execute(w, data); err != nil {
		return fmt.Errorf("export map HTML: %w", err)
	}
	return nil
}
