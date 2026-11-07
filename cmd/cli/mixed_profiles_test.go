package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/qlearning"
)

func TestMixedScenarioProfilesRecordEffectiveConfigAndReports(t *testing.T) {
	text := strings.Replace(trainingYAML, "  scenario: B\n", "", 1)
	text = strings.Replace(text, "    episodes: 12\n", `    profiles:
      - {name: a, scenario: A, episodes: 4}
      - {name: b, scenario: B, episodes: 4}
      - {name: c, scenario: C, episodes: 4}
`, 1)
	text = strings.Replace(text, "seeds: [0, 1]", "runs: 2", 1)
	text = strings.Replace(text, "qlearning:\n", "qlearning:\n  reward:\n    revisit_penalty: -3\n    road:\n      highway:\n        priority: 2\n", 1)
	workdir := newWorkdir(t, text)
	output := filepath.Join(workdir, "output")
	saveSnapshot(t, output, loopSnapshot())
	if err := runTrain(context.Background(), []string{"--workdir", workdir}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	var seeds []uint64
	for _, scenario := range []string{"a", "b", "c"} {
		root := filepath.Join(output, "training", scenario)
		var meta struct {
			Config      qlearning.Config
			Seeds       []uint64 `json:"resolved_seeds"`
			Evaluations []map[string]any
		}
		if err := json.Unmarshal(readFile(t, filepath.Join(root, "metadata.json")), &meta); err != nil {
			t.Fatal(err)
		}
		if string(meta.Config.Scenario) != strings.ToUpper(scenario) || meta.Config.Training.Profiles != nil || meta.Config.Reward.RevisitPenalty != -3 || meta.Config.Reward.Road.Highway.Priority != 2 || len(meta.Evaluations) != 2 {
			t.Fatalf("effective metadata mismatch: %+v", meta)
		}
		if seeds == nil {
			seeds = meta.Seeds
		} else if !reflect.DeepEqual(seeds, meta.Seeds) {
			t.Fatal("profiles do not share automatic seeds")
		}
		rows, err := csv.NewReader(bytes.NewReader(readFile(t, filepath.Join(root, "history.csv")))).ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows[1:] {
			if row[0] != strings.ToUpper(scenario) {
				t.Fatal("scenario not applied to history")
			}
		}
		report := string(readFile(t, filepath.Join(root, "report.html")))
		if !strings.Contains(report, "Training details") || !strings.Contains(report, "revisit_penalty") || !strings.Contains(report, "snapshot_sha256") {
			t.Fatal("report missing provenance")
		}
		dirs, err := filepath.Glob(filepath.Join(root, "seed-*", "metadata.json"))
		if err != nil || len(dirs) != 2 {
			t.Fatalf("seed metadata missing: %v %v", dirs, err)
		}
	}
	// Per-seed metadata includes the exact reward actually used.
	var marker map[string]any
	if err := json.Unmarshal(readFile(t, filepath.Join(output, "training-run.json")), &marker); err != nil {
		t.Fatal(err)
	}
	if marker["status"] != "completed" {
		t.Fatal(marker)
	}
	// Single-profile reruns must not alter an existing sibling report.
	before := readFile(t, filepath.Join(output, "training", "a", "report.html"))
	if err := runTrain(context.Background(), []string{"--workdir", workdir, "--profile", "b"}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(output, "training", "a", "report.html"))
	if !bytes.Equal(before, after) {
		t.Fatal("sibling report changed")
	}
}
