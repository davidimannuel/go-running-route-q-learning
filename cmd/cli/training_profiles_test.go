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
)

func TestTrainingProfilesShareSeedsAndKeepSeparateResults(t *testing.T) {
	content := strings.Replace(trainingYAML, "    episodes: 12\n", `    profiles:
      - name: dynamic-short
        episodes: 4
      - name: dynamic-long
        episodes: 8
      - name: fixed-short
        episodes: 4
        epsilon_decay_episodes: 3
      - name: fixed-long
        episodes: 8
        epsilon_decay_episodes: 3
`, 1)
	content = strings.Replace(content, "seeds: [0, 1]", "runs: 2", 1)
	workdir := newWorkdir(t, content)
	output := filepath.Join(workdir, "output")
	saveSnapshot(t, output, loopSnapshot())
	if err := runTrain(context.Background(), []string{"--workdir", workdir}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Status        string
		ResolvedSeeds []uint64 `json:"resolved_seeds"`
		Profiles      []profileStatus
	}
	if err := json.Unmarshal(readFile(t, filepath.Join(output, "training-run.json")), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Status != "completed" || len(manifest.Profiles) != 4 || len(manifest.ResolvedSeeds) != 2 {
		t.Fatalf("manifest=%+v", manifest)
	}
	for _, profile := range manifest.Profiles {
		if profile.Status != "completed" {
			t.Fatalf("profile status=%s", profile.Status)
		}
		root := filepath.Join(output, "training", profile.Name)
		var metadata struct {
			Profile       string   `json:"training_profile"`
			ResolvedSeeds []uint64 `json:"resolved_seeds"`
			Config        struct {
				Training struct {
					Episodes             int
					EpsilonDecayEpisodes *int `json:"epsilon_decay_episodes"`
				}
			}
		}
		if err := json.Unmarshal(readFile(t, filepath.Join(root, "metadata.json")), &metadata); err != nil {
			t.Fatal(err)
		}
		if metadata.Profile != profile.Name || metadata.Config.Training.Episodes != profile.Episodes || !reflect.DeepEqual(metadata.ResolvedSeeds, manifest.ResolvedSeeds) || !reflect.DeepEqual(metadata.Config.Training.EpsilonDecayEpisodes, profile.EpsilonDecayEpisodes) {
			t.Fatalf("effective config/seed mismatch: %+v", metadata)
		}
		rows, err := csv.NewReader(bytes.NewReader(readFile(t, filepath.Join(root, "history.csv")))).ReadAll()
		if err != nil || len(rows) != 1+2*profile.Episodes {
			t.Fatalf("history rows=%d err=%v", len(rows), err)
		}
		if _, err := os.Stat(filepath.Join(root, "routes.html")); err != nil {
			t.Fatal(err)
		}
	}
	// A selected rerun must replace only that profile; all sibling histories,
	// routes, Q-tables, and completed metadata remain byte-identical.
	other := filepath.Join(output, "training", "dynamic-long")
	before := treeFiles(t, other)
	beforeMetadata := readFile(t, filepath.Join(other, "metadata.json"))
	if err := runTrain(context.Background(), []string{"--workdir", workdir, "--profile", "fixed-short"}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, treeFiles(t, other)) || !bytes.Equal(beforeMetadata, readFile(t, filepath.Join(other, "metadata.json"))) {
		t.Fatal("selected profile overwrote sibling results")
	}
	if err := json.Unmarshal(readFile(t, filepath.Join(output, "training-run.json")), &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Profiles) != 1 || manifest.Profiles[0].Name != "fixed-short" || manifest.Status != "completed" {
		t.Fatalf("selected manifest=%+v", manifest)
	}
	if err := runTrain(context.Background(), []string{"--workdir", workdir, "--profile", "unknown"}, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "unknown profile") {
		t.Fatalf("selection error=%v", err)
	}
}
