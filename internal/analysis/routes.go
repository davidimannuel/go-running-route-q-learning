package analysis

import (
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"strconv"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
	"github.com/davidimannuel/go-running-route-q-learning/internal/qlearning"
)

//go:embed routes.gohtml
var routesHTML string
var routesTemplate = template.Must(template.New("routes").Parse(routesHTML))

type RouteTrial struct {
	Metrics qlearning.Metrics
	Steps   []qlearning.StepResult
}

type routeSegment struct {
	From, To                                     string
	Key, Step                                    int
	WayID, Name, Highway, Surface, Width, Access string
	Coordinates                                  [2][2]float64
	LengthM, BeforeM, AfterM, Reward             float64
	Components                                   qlearning.RewardComponents
	Scores                                       qlearning.RoadScores
}

type routeTrialData struct {
	Seed                                                                     string
	IsLoop                                                                   bool
	Termination                                                              string
	DistanceM, ErrorM, Reward, Comfort, Overlap, ReturnProgress, EndDistance string
	ClosingAvailable                                                         bool
	Steps, Revisits                                                          int
	Segments                                                                 []routeSegment
}

type routeMapData struct {
	Scenario     qlearning.Scenario
	Input, Start [2]float64
	StartNode    string
	SnapM        float64
	Selected     int
	Trials       []routeTrialData
}

// RoutesToHTML follows make_route_map in the reference: kilometer colors,
// start/finish markers, and honest display of failed routes. A selector exposes
// EVERY seed, defaulting to the same representative ranking as the reference.
// Data is embedded with html/template escaping; OSM text is inserted through
// DOM textContent rather than interpreted as popup HTML.
func RoutesToHTML(w io.Writer, graph *osm.Graph, startNode int, lat, lon, snapM float64, trials []RouteTrial, scores qlearning.RoadScoreConfig) error {
	if err := scores.Validate(); err != nil {
		return err
	}

	if err := validateCoordinates(lat, lon); err != nil {
		return err
	}
	start, ok := graph.Nodes[startNode]
	if !ok {
		return fmt.Errorf("route map: start node missing")
	}
	if err := validateCoordinates(start.Lat, start.Lon); err != nil {
		return err
	}
	if len(trials) == 0 {
		return fmt.Errorf("route map: at least one evaluation is required")
	}
	data := routeMapData{Scenario: trials[0].Metrics.Scenario, Input: [2]float64{lat, lon}, Start: [2]float64{start.Lat, start.Lon}, StartNode: strconv.Itoa(startNode), SnapM: snapM}
	rows := make([]qlearning.Metrics, 0, len(trials))
	for _, trial := range trials {
		m := trial.Metrics
		rows = append(rows, m)
		t := routeTrialData{Seed: strconv.FormatUint(m.Seed, 10), IsLoop: m.IsLoop, Termination: string(m.TerminationReason),
			DistanceM: number(m.TotalDistanceM), ErrorM: number(m.AbsoluteDistanceErrorM), Reward: number(m.TotalReward), Comfort: number(m.MeanComfort),
			Overlap: number(m.OverlapRatio), ReturnProgress: number(m.ReturnProgressRatio), EndDistance: number(m.DistanceToStartAtEndM),
			ClosingAvailable: m.ClosingActionAvailable, Steps: m.Steps, Revisits: m.RevisitCount, Segments: make([]routeSegment, 0, len(trial.Steps))}
		cumulative := 0.0
		for _, step := range trial.Steps {
			if !step.ActionTaken {
				continue
			}
			edge := step.Edge
			if edge == nil {
				return fmt.Errorf("route map: action has no edge")
			}
			from, fromOK := graph.Nodes[edge.From]
			to, toOK := graph.Nodes[edge.To]
			if !fromOK || !toOK {
				return fmt.Errorf("route map: edge endpoint missing")
			}
			for _, node := range []osm.Element{from, to} {
				if err := validateCoordinates(node.Lat, node.Lon); err != nil {
					return err
				}
			}
			t.Segments = append(t.Segments, routeSegment{From: integer(edge.From), To: integer(edge.To), Key: edge.Key, Step: step.Step,
				WayID: integer(edge.WayID), Name: edge.Name, Highway: edge.Highway, Surface: edge.Surface, Width: edge.Tags.Get("width"), Access: edge.Access,
				Coordinates: [2][2]float64{{from.Lat, from.Lon}, {to.Lat, to.Lon}}, LengthM: edge.LengthM, BeforeM: cumulative, AfterM: cumulative + edge.LengthM,
				Reward: step.Reward, Components: step.Components, Scores: scores.ScoreEdge(*edge)})
			cumulative += edge.LengthM
		}
		data.Trials = append(data.Trials, t)
	}
	data.Selected = qlearning.Representative(rows)
	if err := routesTemplate.Execute(w, data); err != nil {
		return fmt.Errorf("export route HTML: %w", err)
	}
	return nil
}
