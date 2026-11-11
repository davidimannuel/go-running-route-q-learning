package analysis

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// HTMLOptions changes presentation only. An empty mode preserves legacy debug maps.
type HTMLOptions struct {
	MapMode, IndexURL, SeedSource, QSource string
	Title, Description, DetailsURL         string
	Catalog                                bool
}

func (o *HTMLOptions) normalize() error {
	if o.MapMode == "" {
		o.MapMode = "advanced"
	}
	if o.MapMode != "standard" && o.MapMode != "advanced" {
		return fmt.Errorf("HTML map mode must be standard or advanced")
	}
	if o.IndexURL == "" {
		o.IndexURL = "/"
	}
	if o.DetailsURL == "" {
		o.DetailsURL = "report.html"
	}
	return nil
}

// SeedSource describes actual seed selection, not whether a route is successful.
func SeedSource(automatic bool) string {
	if automatic {
		return "Random seed generated once for the training command and saved for reproduction."
	}
	return "Fixed seed supplied in the training configuration."
}

type SavedRouteSummary struct {
	Scenario string
	Trials   []SiteSeed
}

// ReexportRoutesToHTML reads only the embedded result. It never rebuilds the
// graph or scores with today's config, and never reads or updates a Q-table.
func ReexportRoutesToHTML(w io.Writer, source io.Reader, options HTMLOptions) (SavedRouteSummary, error) {
	var summary SavedRouteSummary
	if err := options.normalize(); err != nil {
		return summary, err
	}
	data, summary, err := readSavedRouteMap(source)
	if err != nil {
		return summary, err
	}
	data.HTML = options
	return summary, routesTemplate.Execute(w, data)
}

func readSavedRouteMap(source io.Reader) (routeMapData, SavedRouteSummary, error) {
	var data routeMapData
	var summary SavedRouteSummary
	if err := readEmbeddedHTML(source, "const routeData = ", ";\n", &data); err != nil {
		return data, summary, err
	}
	if data.Scenario.Validate() != nil || len(data.Trials) == 0 || data.Selected < 0 || data.Selected >= len(data.Trials) || len(data.Graph.Nodes) == 0 {
		return data, summary, fmt.Errorf("saved route HTML has no valid result payload")
	}
	summary.Scenario = string(data.Scenario)
	seen := make(map[string]bool, len(data.Trials))
	for i, trial := range data.Trials {
		if _, err := strconv.ParseUint(trial.Seed, 10, 64); err != nil || seen[trial.Seed] {
			return data, summary, fmt.Errorf("saved route HTML has an invalid or duplicate seed")
		}
		seen[trial.Seed] = true
		summary.Trials = append(summary.Trials, SiteSeed{ID: trial.Seed, Distance: trial.DistanceM, IsLoop: trial.IsLoop, Result: trial.Termination, Representative: i == data.Selected})
	}
	return data, summary, nil
}

// ReexportAnalysisToHTML also preserves the scores and tags of the saved analysis.
func ReexportAnalysisToHTML(w io.Writer, source io.Reader, options HTMLOptions) error {
	if err := options.normalize(); err != nil {
		return err
	}
	var data mapData
	if err := readEmbeddedHTML(source, `<script id="road-data" type="application/json">`, "</script>", &data); err != nil {
		return err
	}
	if len(data.Nodes) == 0 {
		return fmt.Errorf("saved analysis HTML has no graph")
	}
	data.HTML = options
	return roadAnalysisTemplate.Execute(w, data)
}

func readEmbeddedHTML(r io.Reader, start, end string, value any) error {
	// Bound accidental/corrupt input; the current full-graph maps are about 19 MiB.
	const limit = 64 << 20
	body, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return err
	}
	if len(body) > limit {
		return fmt.Errorf("saved HTML exceeds 64 MiB")
	}
	_, rest, ok := strings.Cut(string(body), start)
	if !ok {
		return fmt.Errorf("saved HTML has no supported embedded data")
	}
	payload, _, ok := strings.Cut(rest, end)
	if !ok {
		return fmt.Errorf("saved HTML contains incomplete embedded data")
	}
	if err := json.Unmarshal([]byte(payload), value); err != nil {
		return fmt.Errorf("decode saved HTML data: %w", err)
	}
	return nil
}
