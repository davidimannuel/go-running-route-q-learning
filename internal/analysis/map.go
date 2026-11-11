package analysis

import (
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"math"
	"slices"
	"strconv"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
	"github.com/davidimannuel/go-running-route-q-learning/internal/qlearning"
)

//go:embed road-analysis.gohtml
var roadAnalysisHTML string

//go:embed node-access.gohtml
var nodeAccessHTML string

//go:embed node-inspection.gohtml
var nodeInspectionHTML string

//go:embed q-values.gohtml
var qValuesHTML string

var roadAnalysisTemplate = template.Must(template.New("road-analysis").Parse(roadAnalysisHTML + nodeAccessHTML + nodeInspectionHTML))

type mapNode struct {
	ID            string  `json:"id"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Access        string  `json:"access,omitempty"`
	Barrier       string  `json:"barrier,omitempty"`
	AccessAllowed bool    `json:"access_allowed"`
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
	qlearning.RoadScores
}

// A display segment groups coincident raw edges only. All directed identities,
// parallel actions, attributes, lengths, and scores remain in the edge table.
type mapSegment struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Edges []int  `json:"edges"`
}

type mapData struct {
	HTML     HTMLOptions  `json:"html"`
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
func MapToHTML(w io.Writer, g *osm.Graph, latitude, longitude float64, scores qlearning.RoadScoreConfig) error {
	return MapToHTMLWithOptions(w, g, latitude, longitude, scores, HTMLOptions{})
}

// MapToHTMLWithOptions sets portable navigation without changing graph data.
func MapToHTMLWithOptions(w io.Writer, g *osm.Graph, latitude, longitude float64, scores qlearning.RoadScoreConfig, options HTMLOptions) error {
	if err := options.normalize(); err != nil {
		return err
	}
	data, err := prepareMapData(g, latitude, longitude, scores)
	if err != nil {
		return err
	}
	data.HTML = options
	// html/template serializes this typed value as JSON in script context and
	// escapes <, >, and &. Original OSM strings cannot terminate the data script.
	if err := roadAnalysisTemplate.Execute(w, data); err != nil {
		return fmt.Errorf("export map HTML: %w", err)
	}
	return nil
}

// prepareMapData shares one validated graph payload between analysis and route maps.
// Coincident lines are grouped for display; every directed/parallel edge stays available.
func prepareMapData(g *osm.Graph, latitude, longitude float64, scores qlearning.RoadScoreConfig) (mapData, error) {
	if err := scores.Validate(); err != nil {
		return mapData{}, err
	}

	if err := validateCoordinates(latitude, longitude); err != nil {
		return mapData{}, fmt.Errorf("export map: input location: %w", err)
	}
	data := mapData{
		Center:   [2]float64{latitude, longitude},
		Nodes:    make([]mapNode, 0, len(g.Nodes)),
		Edges:    make([]mapEdge, 0, g.EdgeCount()),
		Segments: make([]mapSegment, 0),
		Weights:  [3]float64{scores.Highway.Weight, scores.Surface.Weight, scores.Width.Weight},
	}
	ids := make([]int, 0, len(g.Nodes))
	for id := range g.Nodes {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		node := g.Nodes[id]
		if err := validateCoordinates(node.Lat, node.Lon); err != nil {
			return mapData{}, fmt.Errorf("export map: node %d: %w", id, err)
		}
		data.Nodes = append(data.Nodes, mapNode{
			ID: strconv.Itoa(id), Latitude: node.Lat, Longitude: node.Lon,
			Access: node.Tags.Get("access"), Barrier: node.Tags.Get("barrier"),
			AccessAllowed: qlearning.AccessAllowed(node.Tags.Get("access")),
		})
	}
	segmentIndex := make(map[[2]int]int)
	for _, edge := range sortedEdges(g) {
		if _, ok := g.Nodes[edge.From]; !ok {
			return mapData{}, fmt.Errorf("export map: edge references missing source node %d", edge.From)
		}
		if _, ok := g.Nodes[edge.To]; !ok {
			return mapData{}, fmt.Errorf("export map: edge references missing destination node %d", edge.To)
		}
		if math.IsNaN(edge.LengthM) || math.IsInf(edge.LengthM, 0) || edge.LengthM < 0 {
			return mapData{}, fmt.Errorf("export map: invalid edge length for (%d, %d, %d)", edge.From, edge.To, edge.Key)
		}
		index := len(data.Edges)
		data.Edges = append(data.Edges, mapEdge{
			From: strconv.Itoa(edge.From), To: strconv.Itoa(edge.To), Key: edge.Key,
			WayID: strconv.Itoa(edge.WayID), LengthM: edge.LengthM,
			Name: edge.Name, Highway: edge.Highway, Surface: edge.Surface,
			Width: edge.Tags.Get("width"), Access: edge.Access, RoadScores: scores.ScoreEdge(edge),
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
	return data, nil
}

// validateCoordinates checks export inputs directly; exporting does not need to
// calculate a distance or depend on the graph loader's private geometry helpers.
func validateCoordinates(latitude, longitude float64) error {
	if math.IsNaN(latitude) || math.IsInf(latitude, 0) || latitude < -90 || latitude > 90 {
		return fmt.Errorf("latitude must be finite and between -90 and 90 degrees")
	}
	if math.IsNaN(longitude) || math.IsInf(longitude, 0) || longitude < -180 || longitude > 180 {
		return fmt.Errorf("longitude must be finite and between -180 and 180 degrees")
	}
	return nil
}
