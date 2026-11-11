package analysis

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"

	"github.com/davidimannuel/go-running-route-q-learning/internal/qlearning"
)

// CatalogEntry records a selected result, independently of future workdir edits.
type CatalogEntry struct {
	ID               string  `json:"id"`
	Description      string  `json:"description"`
	Seed             string  `json:"seed"`
	Profile          string  `json:"profile"`
	Scenario         string  `json:"scenario"`
	DistanceM        float64 `json:"distance_m"`
	AddedAt          string  `json:"added_at"`
	SourceCompleted  string  `json:"source_completed_at"`
	SourceHTMLSHA256 string  `json:"source_html_sha256"`
	MapMode          string  `json:"map_mode"`
	FieldValidation  string  `json:"field_validation"`
}

type CatalogRoute struct {
	data    routeMapData
	Summary SavedRouteSummary
}

// ReadCatalogRoute selects one saved successful seed without recomputing a route.
func ReadCatalogRoute(source io.Reader, seed string) (*CatalogRoute, error) {
	data, summary, err := readSavedRouteMap(source)
	if err != nil {
		return nil, err
	}
	for _, trial := range data.Trials {
		if trial.Seed != seed {
			continue
		}
		if !trial.IsLoop || trial.Termination != string(qlearning.Success) || len(trial.Segments) == 0 {
			return nil, fmt.Errorf("catalog: selected seed is not a saved successful loop")
		}
		data.Trials, data.Selected = []routeTrialData{trial}, 0
		return &CatalogRoute{data: data, Summary: summary}, nil
	}
	return nil, fmt.Errorf("catalog: seed %s is not present in the saved map", seed)
}

func (r *CatalogRoute) DistanceM() float64 {
	distance, _ := strconv.ParseFloat(r.data.Trials[0].DistanceM, 64)
	return distance
}

// ValidateMetadata rejects mixed HTML/metadata and inconsistent loop geometry.
// It verifies saved data only; this does not establish field usability.
func (r *CatalogRoute) ValidateMetadata(metadata map[string]any) error {
	body, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	var recorded struct {
		Status      string           `json:"status"`
		CompletedAt string           `json:"completed_at"`
		Config      qlearning.Config `json:"config"`
		StartNode   int              `json:"start_node"`
		Nodes       int              `json:"nodes"`
		Edges       int              `json:"directed_edges"`
	}
	if err := json.Unmarshal(body, &recorded); err != nil {
		return err
	}
	if recorded.Status != "completed" || recorded.CompletedAt == "" || recorded.Config.Environment.Validate() != nil || string(recorded.Config.Scenario) != r.Summary.Scenario || strconv.Itoa(recorded.StartNode) != r.data.StartNode || recorded.Nodes != len(r.data.Graph.Nodes) || recorded.Edges != len(r.data.Graph.Edges) {
		return fmt.Errorf("catalog: map and completed metadata do not describe the same graph/start/configuration")
	}
	trial := r.data.Trials[0]
	distance := r.DistanceM()
	if !finiteCatalog(distance) || distance < recorded.Config.Environment.MinDistanceM() || distance > recorded.Config.Environment.MaxDistanceM() {
		return fmt.Errorf("catalog: saved distance is outside the recorded valid range")
	}
	current, length := r.data.StartNode, 0.0
	for _, segment := range trial.Segments {
		if segment.From != current || !finiteCatalog(segment.LengthM) || segment.LengthM <= 0 {
			return fmt.Errorf("catalog: saved route has a disconnected or invalid segment")
		}
		current, length = segment.To, length+segment.LengthM
	}
	if current != r.data.StartNode || math.Abs(length-distance) > 1e-6 {
		return fmt.Errorf("catalog: saved route is not closed or its distance differs from its segments")
	}
	rows, _ := metadata["evaluations"].([]any)
	var evaluation map[string]any
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if ok && fmt.Sprint(row["seed"]) == trial.Seed {
			if evaluation != nil {
				return fmt.Errorf("catalog: duplicate saved evaluation")
			}
			evaluation = row
		}
	}
	if evaluation == nil || evaluation["is_loop"] != true || evaluation["termination_reason"] != string(qlearning.Success) {
		return fmt.Errorf("catalog: selected seed has no matching successful evaluation")
	}
	for _, pair := range []struct{ key, text string }{
		{"total_distance_m", trial.DistanceM}, {"absolute_distance_error_m", trial.ErrorM},
		{"total_reward", trial.Reward}, {"mean_comfort", trial.Comfort}, {"overlap_ratio", trial.Overlap},
	} {
		want, err := strconv.ParseFloat(pair.text, 64)
		got, parseErr := strconv.ParseFloat(fmt.Sprint(evaluation[pair.key]), 64)
		if err != nil || parseErr != nil || !finiteCatalog(want) || !finiteCatalog(got) || math.Abs(want-got) > 1e-6 {
			return fmt.Errorf("catalog: selected map and evaluation differ for %s", pair.key)
		}
	}
	return nil
}

func finiteCatalog(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

// ToHTML always forces standard mode, irrespective of the source's map mode.
func (r *CatalogRoute) ToHTML(w io.Writer, entry CatalogEntry, seedSource string) error {
	r.data.HTML = HTMLOptions{MapMode: "standard", IndexURL: "../../index.html", DetailsURL: "details.html", Title: "Route · " + entry.ID, Description: entry.Description, SeedSource: seedSource, Catalog: true}
	return routesTemplate.Execute(w, r.data)
}

func CatalogEntriesToHTML(w io.Writer, entries []CatalogEntry) error {
	items := make([]SiteEntry, 0, len(entries))
	for _, entry := range entries {
		prefix := "routes/" + entry.ID + "/"
		items = append(items, SiteEntry{Name: entry.ID, Description: entry.Description, Kind: "training", Scenario: entry.Scenario, MapURL: prefix + "index.html", DetailsURL: prefix + "details.html", CompletedAt: entry.SourceCompleted,
			Seeds: []SiteSeed{{ID: entry.Seed, Distance: strconv.FormatFloat(entry.DistanceM, 'f', 2, 64), Result: "success", IsLoop: true, URL: prefix + "index.html?seed=" + entry.Seed}}})
	}
	return CatalogToHTML(w, items)
}
