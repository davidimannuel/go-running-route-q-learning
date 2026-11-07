package qlearning

import (
	"fmt"
	"math"
	"math/rand/v2"
)

// StepResult exposes the actual transition and Bellman calculation for manual
// verification. ActionTaken/QUpdated distinguish absent values from valid zero.
type StepResult struct {
	Transition
	Epsilon                                      float64
	RewardStep, StopPenalty, Reward              float64
	QUpdated                                     bool
	QBefore, MaxNextQ, TDTarget, TDError, QAfter float64
}

// chooseAction samples uniformly among near-equal best values, using the
// reference np.isclose defaults (rtol=1e-5, atol=1e-8). It does not prefer the
// first parallel edge. Candidate order is fixed before training begins.
func chooseAction(e *Environment, q QTable, rng *rand.Rand, epsilon float64) (ActionID, bool) {
	actions := e.Available()
	if len(actions) == 0 {
		return ActionID{}, false
	}
	if rng.Float64() < epsilon {
		edge := actions[rng.IntN(len(actions))]
		return ActionID{edge.To, edge.Key}, true
	}
	values := q.values(e.State())
	best := math.Inf(-1)
	for _, edge := range actions {
		best = math.Max(best, values[ActionID{edge.To, edge.Key}])
	}
	count := 0
	for _, edge := range actions {
		if closeQ(values[ActionID{edge.To, edge.Key}], best) {
			count++
		}
	}
	selected := rng.IntN(count)
	for _, edge := range actions {
		id := ActionID{edge.To, edge.Key}
		if closeQ(values[id], best) {
			if selected == 0 {
				return id, true
			}
			selected--
		}
	}
	panic("unreachable action selection")
}
func closeQ(a, b float64) bool { return math.Abs(a-b) <= 1e-8+1e-5*math.Abs(b) }

// advance handles both initial and post-movement dead ends. The latter penalty
// must be included BEFORE the Bellman update; it is never added twice.
func advance(e *Environment, id *ActionID) (StepResult, error) {
	if e.done {
		return StepResult{}, fmt.Errorf("qlearning: episode finished; reset before stepping")
	}
	if id == nil {
		if len(e.Available()) != 0 {
			return StepResult{}, fmt.Errorf("qlearning: an action is required when legal actions exist")
		}
		t := e.stopWithoutAction()
		return StepResult{Transition: t, StopPenalty: e.graph.reward.NoActionPenalty, Reward: e.graph.reward.NoActionPenalty}, nil
	}
	t, err := e.Step(*id)
	if err != nil {
		return StepResult{}, err
	}
	r := StepResult{Transition: t, RewardStep: t.Components.Total()}
	if !e.done && len(e.Available()) == 0 {
		e.stopWithoutAction()
		r.Components.NoAction = e.graph.reward.NoActionPenalty
		r.StopPenalty = e.graph.reward.NoActionPenalty
		r.Done, r.Termination = true, NoAvailableAction
	}
	r.Reward = r.RewardStep + r.StopPenalty
	return r, nil
}

// TrainStep is shared by real training and unit/manual checks. Supplying action
// bypasses policy sampling only; validation, rewards, termination, and the Q
// update still use the same path. Callers must pass a non-nil table and RNG.
func TrainStep(e *Environment, q QTable, rng *rand.Rand, cfg TrainingConfig, epsilon float64, action *ActionID) (StepResult, error) {
	if e == nil || q == nil || rng == nil {
		return StepResult{}, fmt.Errorf("qlearning: environment, Q-table, and RNG are required")
	}
	if !finite(cfg.Alpha) || cfg.Alpha <= 0 || cfg.Alpha > 1 || !finite(cfg.Gamma) || cfg.Gamma < 0 || cfg.Gamma > 1 {
		return StepResult{}, fmt.Errorf("qlearning: invalid alpha or gamma")
	}
	if !finite(epsilon) || epsilon < 0 || epsilon > 1 {
		return StepResult{}, fmt.Errorf("qlearning: epsilon must be in [0,1]")
	}
	if action == nil {
		if id, ok := chooseAction(e, q, rng, epsilon); ok {
			action = &id
		}
	}
	r, err := advance(e, action)
	if err != nil {
		return StepResult{}, err
	}
	r.Epsilon = epsilon
	if !r.ActionTaken {
		return r, nil
	}
	if !r.Done {
		next := e.Available()
		values := q.values(r.NextState)
		r.MaxNextQ = math.Inf(-1)
		for _, edge := range next {
			r.MaxNextQ = math.Max(r.MaxNextQ, values[ActionID{edge.To, edge.Key}])
		}
	}
	values := q.values(r.State)
	r.QBefore = values[r.Action]
	r.TDTarget = r.Reward + cfg.Gamma*r.MaxNextQ
	r.TDError = r.TDTarget - r.QBefore
	r.QAfter = r.QBefore + cfg.Alpha*r.TDError
	values[r.Action] = r.QAfter
	r.QUpdated = true
	return r, nil
}
