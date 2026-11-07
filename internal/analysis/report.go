package analysis

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"strconv"
)

//go:embed report.gohtml
var reportHTML string
var reportTemplate = template.Must(template.New("report").Parse(reportHTML))

type reportField struct{ Name, Value string }
type reportEvaluation struct{ Seed, Scenario, Result, Distance, Error, Reward, Comfort, Overlap string }
type reportData struct {
	Title, Kind, JSON, MapURL string
	ConfigJSON                string
	Evaluations               []reportEvaluation
	Fields                    []reportField
}

// ReportToHTML keeps a readable, self-contained copy of the recorded metadata.
// JSON numbers are preserved as text so 64-bit seeds do not lose precision in JS.
func ReportToHTML(w io.Writer, title, kind string, metadata any) error {
	body, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("report metadata: %w", err)
	}
	var document map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return err
	}
	data := reportData{Title: title, Kind: kind, JSON: string(body), MapURL: "routes.html"}
	if kind == "analysis" {
		data.MapURL = "road-analysis.html"
	}
	if effective, ok := document["config"]; ok {
		body, _ := json.MarshalIndent(effective, "", "  ")
		data.ConfigJSON = string(body)
	}
	if rows, ok := document["evaluations"].([]any); ok {
		for _, raw := range rows {
			row, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			value := func(key string) string {
				if row[key] == nil {
					return "—"
				}
				return fmt.Sprint(row[key])
			}
			formatted := func(key string, digits int) string {
				number, ok := row[key].(json.Number)
				if !ok {
					return value(key)
				}
				f, err := number.Float64()
				if err != nil {
					return value(key)
				}
				return strconv.FormatFloat(f, 'f', digits, 64)
			}
			data.Evaluations = append(data.Evaluations, reportEvaluation{
				Seed: value("seed"), Scenario: value("scenario"), Result: value("termination_reason"),
				Distance: formatted("total_distance_m", 2), Error: formatted("absolute_distance_error_m", 2),
				Reward: formatted("total_reward", 3), Comfort: formatted("mean_comfort", 4), Overlap: formatted("overlap_ratio", 4),
			})
		}
	}
	get := func(keys ...string) string {
		var value any = document
		for _, key := range keys {
			object, ok := value.(map[string]any)
			if !ok {
				return "—"
			}
			value = object[key]
		}
		if value == nil && len(keys) > 0 && keys[len(keys)-1] == "epsilon_decay_episodes" && kind == "training" {
			return "Dynamic (total episodes)"
		}
		if value == nil {
			return "—"
		}
		if text, ok := value.(string); ok && text == "" {
			return "—"
		}
		return fmt.Sprint(value)
	}
	for _, field := range []struct {
		name string
		path []string
	}{
		{"Status", []string{"status"}}, {"Latest attempt (when different)", []string{"latest_attempt", "status"}}, {"Profile", []string{"training_profile"}},
		{"Scenario", []string{"config", "scenario"}}, {"Episodes per seed", []string{"config", "training", "episodes"}},
		{"Epsilon horizon (null = dynamic)", []string{"config", "training", "epsilon_decay_episodes"}},
		{"Resolved seeds", []string{"resolved_seeds"}}, {"Started (UTC)", []string{"started_at"}},
		{"Completed (UTC)", []string{"completed_at"}}, {"Graph nodes", []string{"nodes"}},
		{"Directed edges", []string{"directed_edges"}}, {"Snapshot SHA-256", []string{"snapshot_sha256"}},
	} {
		data.Fields = append(data.Fields, reportField{field.name, get(field.path...)})
	}
	return reportTemplate.Execute(w, data)
}
