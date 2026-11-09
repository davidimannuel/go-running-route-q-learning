package htmlserver

import (
	"bytes"
	"cmp"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
	"path"
	"slices"
	"strconv"
	"time"

	"github.com/davidimannuel/go-running-route-q-learning/internal/analysis"
	"github.com/davidimannuel/go-running-route-q-learning/internal/logger"
	"github.com/davidimannuel/go-running-route-q-learning/internal/qlearning"
)

type qAction struct {
	NextNode string  `json:"next_node"`
	Key      int     `json:"key"`
	Value    float64 `json:"value"`
}

type qState struct {
	PreviousNode string    `json:"previous_node"`
	ProgressBin  int       `json:"progress_bin"`
	Actions      []qAction `json:"actions"`
}

// Visits retain the actual prefix of the saved evaluation. A state alone does
// not contain visit counts, so arbitrary Q-table states cannot expose that mask.
type qVisit struct {
	Step         int            `json:"step"`
	PreviousNode string         `json:"previous_node"`
	ProgressBin  int            `json:"progress_bin"`
	DistanceM    float64        `json:"distance_m"`
	Chosen       *qAction       `json:"chosen"`
	Visited      map[string]int `json:"visited"`
}

type nodeQData struct {
	Seed         string                      `json:"seed"`
	Node         string                      `json:"node"`
	Scenario     qlearning.Scenario          `json:"scenario"`
	Environment  qlearning.EnvironmentConfig `json:"environment"`
	StartNode    string                      `json:"start_node"`
	AccessPolicy string                      `json:"access_policy"`
	CompletedAt  string                      `json:"completed_at"`
	States       []qState                    `json:"states"`
	Visits       []qVisit                    `json:"visits"`
	ReadMS       float64                     `json:"read_ms"`
	QTableBytes  int64                       `json:"qtable_bytes"`
}

// This endpoint exposes a node-sized projection of an adjacent training CSV,
// never arbitrary files. No table is read until this endpoint is requested.
func (h *Handler) serveQValues(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	name, seedText, nodeText := r.URL.Query().Get("file"), r.URL.Query().Get("seed"), r.URL.Query().Get("node")
	seed, seedErr := strconv.ParseUint(seedText, 10, 64)
	node, nodeErr := strconv.Atoi(nodeText)
	if seedErr != nil || nodeErr != nil || node < 0 || strconv.FormatUint(seed, 10) != seedText || strconv.Itoa(node) != nodeText || path.Base(name) != "routes.html" {
		http.Error(w, "Invalid Q-values request", http.StatusBadRequest)
		return
	}
	document, _, err := h.readMetadata(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var metadata struct {
		Config       qlearning.Config `json:"config"`
		Status       string           `json:"status"`
		Seeds        []uint64         `json:"resolved_seeds"`
		StartNode    int              `json:"start_node"`
		AccessPolicy string           `json:"access_policy_version"`
		CompletedAt  string           `json:"completed_at"`
	}
	encoded, err := json.Marshal(document)
	if err == nil {
		err = json.Unmarshal(encoded, &metadata)
	}
	if err != nil || metadata.Status != "completed" || metadata.CompletedAt == "" || metadata.Config.Scenario.Validate() != nil || metadata.Config.Environment.Validate() != nil {
		http.Error(w, "Training metadata is invalid", http.StatusUnprocessableEntity)
		return
	}
	if !slices.Contains(metadata.Seeds, seed) {
		http.NotFound(w, r)
		return
	}
	directory := path.Join(path.Dir(name), "seed-"+seedText)
	qPath, tracePath := path.Join(directory, "qtable.csv"), path.Join(directory, "evaluation-steps.csv")
	if !h.isRegularFile(qPath) || !h.isRegularFile(tracePath) {
		http.NotFound(w, r)
		return
	}
	// Open both files before reading so an atomic training publication cannot
	// switch either handle halfway through a scan. Detect replaced results below.
	qFile, err := h.root.Open(qPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer qFile.Close()
	traceFile, err := h.root.Open(tracePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer traceFile.Close()
	info, err := qFile.Stat()
	if err != nil || info.Size() > 1<<30 {
		http.Error(w, "Q-table is too large or unavailable", http.StatusUnprocessableEntity)
		return
	}
	traceInfo, err := traceFile.Stat()
	if err != nil || traceInfo.Size() > 8<<20 {
		http.Error(w, "Evaluation trace is too large or unavailable", http.StatusUnprocessableEntity)
		return
	}
	table, err := analysis.ReadNodeQTable(r.Context(), qFile, node)
	if err != nil {
		logger.Debug(r.Context(), "Could not read node Q-values", logger.Err(err))
		http.Error(w, "Could not read saved Q-values", http.StatusUnprocessableEntity)
		return
	}
	visits, err := readNodeVisits(r.Context(), traceFile, nodeText, metadata.Config.Scenario, metadata.StartNode)
	if err != nil {
		logger.Debug(r.Context(), "Could not read evaluation trace", logger.Err(err))
		http.Error(w, "Could not read saved evaluation trace", http.StatusUnprocessableEntity)
		return
	}
	data := nodeQData{Seed: seedText, Node: nodeText, Scenario: metadata.Config.Scenario,
		Environment: metadata.Config.Environment, StartNode: strconv.Itoa(metadata.StartNode),
		AccessPolicy: metadata.AccessPolicy, CompletedAt: metadata.CompletedAt,
		States: make([]qState, 0, len(table)), Visits: visits, QTableBytes: info.Size()}
	for state, actions := range table {
		if state.Scenario != data.Scenario {
			http.Error(w, "Q-table scenario differs from metadata", http.StatusConflict)
			return
		}
		s := qState{PreviousNode: strconv.Itoa(state.PreviousNode), ProgressBin: state.ProgressBin, Actions: make([]qAction, 0, len(actions))}
		for id, value := range actions {
			s.Actions = append(s.Actions, qAction{NextNode: strconv.Itoa(id.NextNode), Key: id.Key, Value: value})
		}
		slices.SortFunc(s.Actions, func(a, b qAction) int {
			x, _ := strconv.Atoi(a.NextNode)
			y, _ := strconv.Atoi(b.NextNode)
			if x != y {
				return cmp.Compare(x, y)
			}
			return cmp.Compare(a.Key, b.Key)
		})
		data.States = append(data.States, s)
	}
	slices.SortFunc(data.States, func(a, b qState) int {
		x, _ := strconv.Atoi(a.PreviousNode)
		y, _ := strconv.Atoi(b.PreviousNode)
		if x != y {
			return cmp.Compare(x, y)
		}
		return cmp.Compare(a.ProgressBin, b.ProgressBin)
	})
	// A chosen Q observation must agree with the table being inspected.
	for _, visit := range visits {
		if visit.Chosen == nil {
			continue
		}
		previous, _ := strconv.Atoi(visit.PreviousNode)
		next, _ := strconv.Atoi(visit.Chosen.NextNode)
		state := qlearning.State{Scenario: data.Scenario, CurrentNode: node, PreviousNode: previous, ProgressBin: visit.ProgressBin}
		if table.Value(state, qlearning.ActionID{NextNode: next, Key: visit.Chosen.Key}) != visit.Chosen.Value {
			http.Error(w, "Saved trace and Q-table differ; reload completed results", http.StatusConflict)
			return
		}
	}
	current, _, err := h.readMetadata(name)
	if err != nil || current["completed_at"] != document["completed_at"] {
		http.Error(w, "Training results changed; reload the map", http.StatusConflict)
		return
	}
	data.ReadMS = float64(time.Since(started).Microseconds()) / 1000
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(data); err != nil {
		http.Error(w, "Could not encode Q-values", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	logger.Debug(r.Context(), "Node Q-values loaded", logger.String("node", nodeText), logger.String("seed", seedText))
	if r.Method != http.MethodHead {
		_, _ = w.Write(body.Bytes())
	}
}

func readNodeVisits(ctx context.Context, r io.Reader, node string, scenario qlearning.Scenario, startNode int) ([]qVisit, error) {
	reader := csv.NewReader(r)
	reader.ReuseRecord = true
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	columns := make(map[string]int, len(header))
	for i, name := range header {
		columns[name] = i
	}
	for _, required := range []string{"step", "scenario", "current_node", "previous_node", "progress_bin", "action_taken", "next_node", "key", "total_distance_m", "q_before"} {
		if _, ok := columns[required]; !ok {
			return nil, fmt.Errorf("trace: missing %s", required)
		}
	}
	visits := make([]qVisit, 0)
	seen := map[string]int{strconv.Itoa(startNode): 1}
	current, distance := strconv.Itoa(startNode), 0.0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		get := func(name string) string { return row[columns[name]] }
		step, e1 := strconv.Atoi(get("step"))
		bin, e2 := strconv.Atoi(get("progress_bin"))
		taken, e3 := strconv.ParseBool(get("action_taken"))
		total, e4 := strconv.ParseFloat(get("total_distance_m"), 64)
		previous, e5 := strconv.Atoi(get("previous_node"))
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil || math.IsNaN(total) || math.IsInf(total, 0) || total < distance || get("current_node") != current || get("scenario") != string(scenario) {
			return nil, fmt.Errorf("trace: invalid transition")
		}
		visit := qVisit{Step: step, PreviousNode: strconv.Itoa(previous), ProgressBin: bin, DistanceM: distance}
		if taken {
			key, e1 := strconv.Atoi(get("key"))
			value, e2 := strconv.ParseFloat(get("q_before"), 64)
			next, e3 := strconv.Atoi(get("next_node"))
			if e1 != nil || e2 != nil || e3 != nil || key < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
				return nil, fmt.Errorf("trace: invalid action")
			}
			visit.Chosen = &qAction{NextNode: strconv.Itoa(next), Key: key, Value: value}
		}
		if current == node {
			visit.Visited = maps.Clone(seen)
			visits = append(visits, visit)
		}
		if taken {
			current = visit.Chosen.NextNode
			seen[current]++
		}
		distance = total
	}
	return visits, nil
}
