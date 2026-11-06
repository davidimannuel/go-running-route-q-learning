package qlearning

import "math"

// Metrics is used unchanged for episode histories and final greedy evaluations.
// Undefined ratios use NaN (CSV), not zero observations disguised as a result.
type Metrics struct {
	Scenario                  Scenario
	Seed                      uint64
	Episode                   int
	Epsilon                   float64
	IsLoop                    bool
	TerminationReason         Termination
	TotalDistanceM            float64
	AbsoluteDistanceErrorM    float64
	OvershootM                float64
	DistanceToStartAtEndM     float64
	TotalReward               float64
	MeanComfort               float64
	OverlapRatio              float64
	ReturnProgressRatio       float64
	ReturnPhaseSteps          int
	ReturnProgressSteps       int
	ClosingActionAvailable    bool
	Steps                     int
	RevisitCount              int
	RepeatedIntermediateNodes int
	UniqueEdgeRatio           float64
	QStateCount               int
	MeanAbsTDError            float64
}

func (e *Environment) Metrics(reward float64) Metrics {
	m := Metrics{Scenario: e.scenario, IsLoop: e.isLoop, TerminationReason: e.reason,
		TotalDistanceM: e.distance, AbsoluteDistanceErrorM: math.Abs(e.distance - e.config.TargetDistanceM),
		OvershootM: math.Max(0, e.distance-e.config.MaxDistanceM()), DistanceToStartAtEndM: e.graph.distances[e.current],
		TotalReward: reward, MeanComfort: math.NaN(), OverlapRatio: math.NaN(), ReturnProgressRatio: math.NaN(),
		ClosingActionAvailable: e.closingAvailable, Steps: e.steps, RevisitCount: e.revisits,
		ReturnPhaseSteps: e.returnSteps, ReturnProgressSteps: e.returnProgress, MeanAbsTDError: math.NaN()}
	if e.returnSteps > 0 {
		m.ReturnProgressRatio = float64(e.returnProgress) / float64(e.returnSteps)
	}
	if len(e.path) > 0 {
		var weighted, length float64
		unique := e.metricEdges
		clear(unique)
		intermediate := e.metricNodes
		clear(intermediate)
		for i, edge := range e.path {
			weighted += e.graph.edges[edge].comfort * edge.LengthM
			length += edge.LengthM
			unique[[3]int{edge.From, edge.To, edge.Key}] = struct{}{}
			if i < len(e.path)-1 {
				intermediate[edge.To] = struct{}{}
			}
		}
		m.MeanComfort = weighted / length
		m.OverlapRatio = float64(len(e.path)-len(e.roads)) / float64(len(e.path))
		m.UniqueEdgeRatio = float64(len(unique)) / float64(len(e.path))
		m.RepeatedIntermediateNodes = max(0, len(e.path)-1) - len(intermediate)
	}
	return m
}

// Representative uses the reference ranking; failures are still reportable.
func Representative(rows []Metrics) int {
	best := -1
	rankValue := func(v float64) float64 {
		if math.IsNaN(v) {
			return -1
		}
		return v
	}
	better := func(a, b Metrics) bool {
		if a.IsLoop != b.IsLoop {
			return a.IsLoop
		}
		if a.AbsoluteDistanceErrorM != b.AbsoluteDistanceErrorM {
			return a.AbsoluteDistanceErrorM < b.AbsoluteDistanceErrorM
		}
		if !a.IsLoop && a.DistanceToStartAtEndM != b.DistanceToStartAtEndM {
			return a.DistanceToStartAtEndM < b.DistanceToStartAtEndM
		}
		if rankValue(a.MeanComfort) != rankValue(b.MeanComfort) {
			return rankValue(a.MeanComfort) > rankValue(b.MeanComfort)
		}
		if rankValue(a.ReturnProgressRatio) != rankValue(b.ReturnProgressRatio) {
			return rankValue(a.ReturnProgressRatio) > rankValue(b.ReturnProgressRatio)
		}
		return a.Seed < b.Seed
	}
	for i, row := range rows {
		if best < 0 || better(row, rows[best]) {
			best = i
		}
	}
	return best
}
