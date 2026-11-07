package qlearning

import (
	"fmt"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
)

type Termination string

const (
	Running           Termination = "running"
	Success           Termination = "success"
	OverDistance      Termination = "over_distance"
	NoAvailableAction Termination = "no_available_action"
	StepLimit         Termination = "step_limit"
)

// Environment owns one episode's mutable state. Reset reuses bounded buffers;
// it never resets the seed RNG or Q-table. Use one environment per worker.
type Environment struct {
	graph            *PreparedGraph
	scenario         Scenario
	config           EnvironmentConfig
	current          int
	previous         int
	steps            int
	revisits         int
	distance         float64
	visits           map[int]int
	roads            map[int]struct{}
	path             []*osm.OSMEdge
	fresh            []*osm.OSMEdge
	revisit          []*osm.OSMEdge
	closing          []*osm.OSMEdge
	legal            []*osm.OSMEdge
	returnSteps      int
	returnProgress   int
	closingAvailable bool
	done             bool
	isLoop           bool
	reason           Termination
	// Metrics reuse scratch sets rather than allocate two maps per episode.
	metricEdges map[[3]int]struct{}
	metricNodes map[int]struct{}
}

func NewEnvironment(p *PreparedGraph, scenario Scenario, cfg EnvironmentConfig) (*Environment, error) {
	if p == nil {
		return nil, fmt.Errorf("environment: prepared graph is nil")
	}
	if err := scenario.Validate(); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	e := &Environment{graph: p, scenario: scenario, config: cfg, visits: make(map[int]int), roads: make(map[int]struct{})}
	e.metricEdges = make(map[[3]int]struct{})
	e.metricNodes = make(map[int]struct{})
	e.Reset()
	return e, nil
}

func (e *Environment) Reset() State {
	e.current, e.previous = e.graph.startNode, -1
	e.steps, e.revisits, e.distance = 0, 0, 0
	clear(e.visits)
	e.visits[e.current] = 1
	clear(e.roads)
	e.path = e.path[:0]
	e.returnSteps, e.returnProgress = 0, 0
	e.closingAvailable, e.done, e.isLoop = false, false, false
	e.reason = Running
	return e.State()
}

func (e *Environment) State() State {
	state := State{Scenario: e.scenario, CurrentNode: e.current, PreviousNode: -1}
	if e.scenario == ScenarioB || e.scenario == ScenarioC {
		bin := int(e.distance / e.config.MaxDistanceM() * float64(e.config.ProgressBins))
		if bin < 0 {
			bin = 0
		}
		if bin >= e.config.ProgressBins {
			bin = e.config.ProgressBins - 1
		}
		state.ProgressBin = bin
	}
	if e.scenario == ScenarioC {
		state.PreviousNode = e.previous
	}
	return state
}

// Available applies the same mask for exploration, greedy choice, and bootstrap.
// The returned scratch slice is read-only and valid until the next call/reset.
// Preserve reference order: fresh, then permitted revisits, then closing edges.
func (e *Environment) Available() []*osm.OSMEdge {
	e.fresh, e.revisit, e.closing, e.legal = e.fresh[:0], e.revisit[:0], e.closing[:0], e.legal[:0]
	if e.done {
		return e.legal
	}
	for _, edge := range e.graph.actions.Available(e.current) {
		if edge.To == e.graph.startNode {
			nextDistance := e.distance + edge.LengthM
			if e.current != e.graph.startNode && nextDistance >= e.config.MinDistanceM() && nextDistance <= e.config.MaxDistanceM() {
				e.closing = append(e.closing, edge)
			}
			continue // Start never falls through to ordinary revisit/fresh rules.
		}
		if e.visits[edge.To] == 0 {
			e.fresh = append(e.fresh, edge)
			continue
		}
		if e.config.AllowRevisit && e.distance >= e.config.RevisitAfterRatio*e.config.TargetDistanceM && e.visits[edge.To] < e.config.MaxNodeVisits {
			e.revisit = append(e.revisit, edge)
		}
	}
	if len(e.closing) > 0 {
		e.closingAvailable = true
	}
	e.legal = append(e.legal, e.fresh...)
	e.legal = append(e.legal, e.revisit...)
	e.legal = append(e.legal, e.closing...)
	return e.legal
}

type Transition struct {
	State, NextState                                State
	Action                                          ActionID
	ActionTaken                                     bool
	Edge                                            *osm.OSMEdge
	Components                                      RewardComponents
	DistanceBeforeM, DistanceAfterM, TotalDistanceM float64
	Step                                            int
	IsRevisit, Done                                 bool
	Termination                                     Termination
	DirectionRule                                   string
}

func (e *Environment) transition() Transition {
	return Transition{State: e.State(), NextState: e.State(), TotalDistanceM: e.distance, Step: e.steps,
		Done: e.done, Termination: e.reason, DirectionRule: "not_applied"}
}

// Step resolves the requested identity to the actual legal graph edge. A caller
// cannot substitute fabricated lengths/tags by supplying another edge value.
func (e *Environment) Step(id ActionID) (Transition, error) {
	if e.done {
		return Transition{}, fmt.Errorf("environment: episode finished; reset before stepping")
	}
	var edge *osm.OSMEdge
	for _, candidate := range e.Available() {
		if candidate.To == id.NextNode && candidate.Key == id.Key {
			edge = candidate
			break
		}
	}
	if edge == nil {
		return Transition{}, fmt.Errorf("environment: illegal action (%d,%d)", id.NextNode, id.Key)
	}
	t := e.transition()
	t.Action, t.ActionTaken, t.Edge = id, true, edge
	t.DistanceBeforeM = e.graph.distances[e.current]
	outward := e.distance < e.graph.reward.ReturnPhaseRatio*e.config.TargetDistanceM
	t.IsRevisit = e.visits[edge.To] > 0
	e.previous, e.current = e.current, edge.To
	e.distance += edge.LengthM
	e.steps++
	e.visits[e.current]++
	e.path = append(e.path, edge)
	data := e.graph.edges[edge]
	e.roads[data.roadID] = struct{}{}
	t.DistanceAfterM = e.graph.distances[e.current]
	returning := e.distance >= e.graph.reward.ReturnPhaseRatio*e.config.TargetDistanceM
	if returning {
		e.returnSteps++
		if t.DistanceAfterM < t.DistanceBeforeM {
			e.returnProgress++
		}
	}
	t.Components.Edge = data.comfort * (edge.LengthM / 100)
	t.Components.Access = data.accessReward
	if t.IsRevisit && e.current != e.graph.startNode {
		e.revisits++
		t.Components.Revisit = e.graph.reward.RevisitPenalty
	}
	// Terminal priority and phase boundary follow core.step exactly. In
	// particular, terminal actions have no direction contribution, and an edge
	// crossing 2,500 m can still receive the early-phase approach penalty.
	switch {
	case e.current == e.graph.startNode && e.distance >= e.config.MinDistanceM() && e.distance <= e.config.MaxDistanceM():
		e.done, e.isLoop, e.reason = true, true, Success
		t.Components.Terminal = e.graph.reward.SuccessBonus
	case e.distance > e.config.MaxDistanceM():
		e.done, e.reason = true, OverDistance
		t.Components.Terminal = e.graph.reward.OverDistancePenalty
	case e.steps >= e.config.MaxSteps:
		e.done, e.reason = true, StepLimit
		t.Components.Terminal = e.graph.reward.StepLimitPenalty
	case outward && t.DistanceAfterM < t.DistanceBeforeM:
		t.Components.Direction = -e.graph.reward.OutwardPenaltyScale * (t.DistanceBeforeM - t.DistanceAfterM) / 100
		t.DirectionRule = "outward_approach_penalty"
	case returning:
		t.Components.Direction = e.graph.reward.ReturnRewardScale * (t.DistanceBeforeM - t.DistanceAfterM) / 100
		t.DirectionRule = "return_progress"
	}
	t.NextState, t.TotalDistanceM, t.Step, t.Done, t.Termination = e.State(), e.distance, e.steps, e.done, e.reason
	return t, nil
}

func (e *Environment) stopWithoutAction() Transition {
	t := e.transition()
	e.done, e.reason = true, NoAvailableAction
	t.Done, t.Termination = true, NoAvailableAction
	t.Components.NoAction = e.graph.reward.NoActionPenalty
	return t
}
