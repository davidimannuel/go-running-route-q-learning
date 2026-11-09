package analysis

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
	"github.com/davidimannuel/go-running-route-q-learning/internal/qlearning"
)

func number(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
func integer(v int) string    { return strconv.Itoa(v) }
func boolean(v bool) string   { return strconv.FormatBool(v) }

// MetricsCSV streams rows; it never retains an entire training history.
// CSV NaN values are intentionally readable as missing data by pandas.
type MetricsCSV struct{ writer *csv.Writer }

func NewMetricsCSV(w io.Writer) (*MetricsCSV, error) {
	writer := csv.NewWriter(w)
	err := writer.Write([]string{"scenario", "seed", "episode", "epsilon", "is_loop", "termination_reason",
		"total_distance_m", "distance_km", "absolute_distance_error_m", "overshoot_m", "total_reward", "mean_comfort",
		"overlap_ratio", "return_progress_ratio", "distance_to_start_at_end_m", "closing_action_available",
		"steps", "revisit_count", "return_phase_steps", "return_progress_steps", "repeated_intermediate_nodes",
		"unique_edge_ratio", "q_state_count", "mean_abs_td_error"})
	if err != nil {
		return nil, err
	}
	return &MetricsCSV{writer: writer}, nil
}
func (w *MetricsCSV) Write(m qlearning.Metrics) error {
	return w.writer.Write([]string{string(m.Scenario), strconv.FormatUint(m.Seed, 10), integer(m.Episode), number(m.Epsilon), boolean(m.IsLoop), string(m.TerminationReason),
		number(m.TotalDistanceM), number(m.TotalDistanceM / 1000), number(m.AbsoluteDistanceErrorM), number(m.OvershootM), number(m.TotalReward), number(m.MeanComfort),
		number(m.OverlapRatio), number(m.ReturnProgressRatio), number(m.DistanceToStartAtEndM), boolean(m.ClosingActionAvailable),
		integer(m.Steps), integer(m.RevisitCount), integer(m.ReturnPhaseSteps), integer(m.ReturnProgressSteps), integer(m.RepeatedIntermediateNodes),
		number(m.UniqueEdgeRatio), integer(m.QStateCount), number(m.MeanAbsTDError)})
}
func (w *MetricsCSV) Flush() error { w.writer.Flush(); return w.writer.Error() }

var qHeader = []string{"scenario", "current_node", "previous_node", "progress_bin", "next_node", "key", "q_value"}

// QTableToCSV saves empty state entries too, matching reference diagnostic counts.
// Stable ordering makes snapshots reviewable and independent of map iteration.
func QTableToCSV(w io.Writer, table qlearning.QTable) error {
	writer := csv.NewWriter(w)
	if err := writer.Write(qHeader); err != nil {
		return err
	}
	states := make([]qlearning.State, 0, len(table))
	for state := range table {
		states = append(states, state)
	}
	slices.SortFunc(states, func(a, b qlearning.State) int {
		if a.Scenario < b.Scenario {
			return -1
		}
		if a.Scenario > b.Scenario {
			return 1
		}
		for _, pair := range [][2]int{{a.CurrentNode, b.CurrentNode}, {a.PreviousNode, b.PreviousNode}, {a.ProgressBin, b.ProgressBin}} {
			if pair[0] < pair[1] {
				return -1
			}
			if pair[0] > pair[1] {
				return 1
			}
		}
		return 0
	})
	for _, state := range states {
		prefix := []string{string(state.Scenario), integer(state.CurrentNode), integer(state.PreviousNode), integer(state.ProgressBin)}
		actions := make([]qlearning.ActionID, 0, len(table[state]))
		for id := range table[state] {
			actions = append(actions, id)
		}
		slices.SortFunc(actions, func(a, b qlearning.ActionID) int {
			if a.NextNode < b.NextNode {
				return -1
			}
			if a.NextNode > b.NextNode {
				return 1
			}
			if a.Key < b.Key {
				return -1
			}
			if a.Key > b.Key {
				return 1
			}
			return 0
		})
		if len(actions) == 0 {
			if err := writer.Write(append(prefix, "", "", "")); err != nil {
				return err
			}
		}
		for _, id := range actions {
			value := table[state][id]
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return fmt.Errorf("export Q-table: non-finite Q-value")
			}
			if err := writer.Write(append(prefix, integer(id.NextNode), integer(id.Key), number(value))); err != nil {
				return err
			}
		}
	}
	writer.Flush()
	return writer.Error()
}

// ReadQTable reloads the Go CSV for independent greedy reproduction without
// training again. Node/action zero remain valid; blank action fields mean an
// empty state entry, never a synthetic action (0,0).
func ReadQTable(r io.Reader) (qlearning.QTable, error) {
	return readQTable(context.Background(), r, nil)
}

// ReadNodeQTable streams the CSV and retains only one node's states/actions.
// It does not assume row ordering or load the other nodes into memory.
func ReadNodeQTable(ctx context.Context, r io.Reader, node int) (qlearning.QTable, error) {
	return readQTable(ctx, r, &node)
}

func readQTable(ctx context.Context, r io.Reader, node *int) (qlearning.QTable, error) {
	reader := csv.NewReader(r)
	reader.ReuseRecord = true
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read Q-table header: %w", err)
	}
	if !slices.Equal(header, qHeader) {
		return nil, fmt.Errorf("read Q-table: unexpected columns")
	}
	table := make(qlearning.QTable)
	nodeText := ""
	if node != nil {
		nodeText = strconv.Itoa(*node)
	}
	for rowNumber := 2; ; rowNumber++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read Q-table row %d: %w", rowNumber, err)
		}
		if node != nil && row[1] != nodeText {
			continue
		}
		state := qlearning.State{Scenario: qlearning.Scenario(row[0])}
		if err := state.Scenario.Validate(); err != nil {
			return nil, err
		}
		fields := []*int{&state.CurrentNode, &state.PreviousNode, &state.ProgressBin}
		for i, field := range fields {
			value, err := strconv.Atoi(row[i+1])
			if err != nil {
				return nil, fmt.Errorf("read Q-table row %d: invalid state", rowNumber)
			}
			*field = value
		}
		if table[state] == nil {
			table[state] = make(map[qlearning.ActionID]float64)
		}
		if row[4] == "" && row[5] == "" && row[6] == "" {
			continue
		}
		next, e1 := strconv.Atoi(row[4])
		key, e2 := strconv.Atoi(row[5])
		value, e3 := strconv.ParseFloat(row[6], 64)
		if e1 != nil || e2 != nil || e3 != nil || key < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("read Q-table row %d: invalid action/value", rowNumber)
		}
		id := qlearning.ActionID{NextNode: next, Key: key}
		if _, exists := table[state][id]; exists {
			return nil, fmt.Errorf("read Q-table row %d: duplicate action", rowNumber)
		}
		table[state][id] = value
	}
	return table, nil
}

// StepsToCSV exports the actual greedy transitions and reward components. Q
// diagnostics in evaluation are observations: q_updated is always false.
func StepsToCSV(w io.Writer, graph *osm.Graph, steps []qlearning.StepResult) error {
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"step", "scenario", "current_node", "previous_node", "progress_bin", "action_taken", "next_node", "key",
		"next_previous_node", "next_progress_bin", "from_lat", "from_lon", "to_lat", "to_lon", "way_id", "name", "highway", "surface", "width", "access",
		"length_m", "total_distance_m", "straight_distance_before_m", "straight_distance_after_m", "is_revisit", "direction_rule",
		"reward_edge", "reward_access", "reward_direction", "reward_revisit", "reward_terminal", "reward_no_action", "reward_step", "stop_penalty", "reward",
		"done", "termination_reason", "q_updated", "q_before", "max_next_q", "td_target", "td_error", "q_after"}); err != nil {
		return err
	}
	for _, s := range steps {
		from, to := graph.Nodes[s.State.CurrentNode], graph.Nodes[s.NextState.CurrentNode]
		next, key, way, name, highway, surface, width, access, length := "", "", "", "", "", "", "", "", ""
		if s.ActionTaken {
			e := s.Edge
			next, key, way = integer(e.To), integer(e.Key), integer(e.WayID)
			name, highway, surface, width, access, length = e.Name, e.Highway, e.Surface, e.Tags.Get("width"), e.Access, number(e.LengthM)
		}
		qBefore, qAfter := "", ""
		if s.ActionTaken {
			qBefore, qAfter = number(s.QBefore), number(s.QAfter)
		}
		maxNext, target, td := "", "", ""
		if s.QUpdated {
			maxNext, target, td = number(s.MaxNextQ), number(s.TDTarget), number(s.TDError)
		}
		c := s.Components
		if err := writer.Write([]string{integer(s.Step), string(s.State.Scenario), integer(s.State.CurrentNode), integer(s.State.PreviousNode), integer(s.State.ProgressBin), boolean(s.ActionTaken), next, key,
			integer(s.NextState.PreviousNode), integer(s.NextState.ProgressBin), number(from.Lat), number(from.Lon), number(to.Lat), number(to.Lon), way, name, highway, surface, width, access,
			length, number(s.TotalDistanceM), number(s.DistanceBeforeM), number(s.DistanceAfterM), boolean(s.IsRevisit), s.DirectionRule,
			number(c.Edge), number(c.Access), number(c.Direction), number(c.Revisit), number(c.Terminal), number(c.NoAction), number(s.RewardStep), number(s.StopPenalty), number(s.Reward),
			boolean(s.Done), string(s.Termination), boolean(s.QUpdated), qBefore, maxNext, target, td, qAfter}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}
