package qlearning

// State is a comparable Q-table key. Scenario identifies active fields, so a
// valid bin/node zero is never confused with an unused field. PreviousNode=-1
// means no previous node (reset), or a field unused by scenarios A/B.
type State struct {
	Scenario     Scenario
	CurrentNode  int
	PreviousNode int
	ProgressBin  int
}

type ActionID struct{ NextNode, Key int }

type QTable map[State]map[ActionID]float64

// Value reads an unseen state/action as zero without allocating a table entry.
func (q QTable) Value(state State, action ActionID) float64 { return q[state][action] }

func (q QTable) values(state State) map[ActionID]float64 {
	values := q[state]
	if values == nil {
		values = make(map[ActionID]float64)
		q[state] = values
	}
	return values
}
