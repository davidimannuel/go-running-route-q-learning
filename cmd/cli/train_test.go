package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/analysis"
	"github.com/davidimannuel/go-running-route-q-learning/internal/config"
	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
	"github.com/davidimannuel/go-running-route-q-learning/internal/qlearning"
)

const trainingYAML = `schema_version: 1
map:
  latitude: 0
  longitude: 0
  radius_m: 3025
  timeout_sec: 90
qlearning:
  scenario: B
  training:
    episodes: 12
    seeds: [0, 1]
    max_workers: 1
`

func loopSnapshot() osm.OSMData {
	return osm.OSMData{Version: 0.6, Elements: []osm.Element{
		{Type: osm.TypeNode, ID: 0, Lat: 0, Lon: 0},
		{Type: osm.TypeNode, ID: 1, Lat: 0, Lon: 0.015},
		{Type: osm.TypeNode, ID: 2, Lat: 0.013, Lon: 0.0075},
		{Type: osm.TypeWay, ID: 100, Nodes: []int{0, 1, 2, 0}, Tags: osm.Tags{"highway": "residential", "surface": "asphalt", "width": "3", "access": "permissive", "name": "Local test road"}},
	}}
}
func treeFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	result := make(map[string][]byte)
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if filepath.Base(relative) != "metadata.json" && relative != "report.html" {
			result[relative] = readFile(t, path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestTrainCLIArtifactsParallelReproductionAndReload(t *testing.T) {
	workdir := newWorkdir(t, trainingYAML)
	output := filepath.Join(workdir, "output")
	original := saveSnapshot(t, output, loopSnapshot())
	var serial map[string][]byte
	for _, workers := range []int{1, 2} {
		content := strings.Replace(trainingYAML, "max_workers: 1", fmt.Sprintf("max_workers: %d", workers), 1)
		if err := os.WriteFile(filepath.Join(workdir, "config.yaml"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		var logs, stdout bytes.Buffer
		fetcher := &fakeFetcher{err: fmt.Errorf("training must not download")}
		if err := run(context.Background(), []string{"train", "--workdir", workdir}, &stdout, &logs, fetcher); err != nil {
			t.Fatal(err)
		}
		if fetcher.calls != 0 {
			t.Fatal("training downloaded data")
		}
		root := filepath.Join(output, "training")
		files := treeFiles(t, root)
		if workers == 1 {
			serial = files
		} else if !reflect.DeepEqual(serial, files) {
			t.Fatal("parallel and serial artifacts differ")
		}
		rows, err := csv.NewReader(bytes.NewReader(files["history.csv"])).ReadAll()
		if err != nil || len(rows) != 25 {
			t.Fatalf("history rows=%d err=%v", len(rows), err)
		}
		for i, row := range rows[1:] {
			wantSeed := 0
			if i >= 12 {
				wantSeed = 1
			}
			if row[1] != fmt.Sprint(wantSeed) {
				t.Fatal("history not in input seed order")
			}
		}
		if !strings.Contains(stdout.String(), "Summary: greedy loops") || logs.Len() != 0 {
			t.Fatalf("unexpected logs: %s", logs.String())
		}
		if !bytes.Equal(original, readFile(t, filepath.Join(output, "osm.json"))) {
			t.Fatal("snapshot was modified")
		}
		var metadata map[string]any
		if err := json.Unmarshal(readFile(t, filepath.Join(output, "training-run.json")), &metadata); err != nil {
			t.Fatal(err)
		}
		if metadata["status"] != "completed" || metadata["graph_simplified"] != false {
			t.Fatalf("metadata=%v", metadata)
		}
		// Reload Q-table and reproduce the final route/reward, without training.
		cfg, err := config.Load(workdir)
		if err != nil {
			t.Fatal(err)
		}
		graph := osm.NewGraph()
		if err := graph.LoadOSM(loopSnapshot()); err != nil {
			t.Fatal(err)
		}
		p, err := qlearning.NewPreparedGraph(graph, 0, qlearning.DefaultRewardConfig())
		if err != nil {
			t.Fatal(err)
		}
		for _, seed := range []uint64{0, 1} {
			directory := fmt.Sprintf("seed-%d", seed)
			table, err := analysis.ReadQTable(bytes.NewReader(files[filepath.Join(directory, "qtable.csv")]))
			if err != nil {
				t.Fatal(err)
			}
			metrics, steps, err := qlearning.Evaluate(context.Background(), p, cfg.QLearning, seed, table)
			if err != nil {
				t.Fatal(err)
			}
			if !metrics.IsLoop {
				t.Fatal("synthetic triangle must close a valid loop")
			}
			var trace bytes.Buffer
			if err := analysis.StepsToCSV(&trace, graph, steps); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(trace.Bytes(), files[filepath.Join(directory, "evaluation-steps.csv")]) {
				t.Fatal("saved Q-table did not reproduce greedy trace")
			}
		}
	}
	// A subsequent seed list replaces the generated training directory, removing
	// obsolete seed artifacts while keeping unrelated output files untouched.
	if err := os.WriteFile(filepath.Join(output, "keep.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workdir, "config.yaml"), []byte(strings.Replace(trainingYAML, "seeds: [0, 1]", "seeds: [0]", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runTrain(context.Background(), []string{"--workdir", workdir}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(output, "training", "seed-1")); !os.IsNotExist(err) {
		t.Fatal("obsolete seed folder retained")
	}
	if string(readFile(t, filepath.Join(output, "keep.txt"))) != "keep" {
		t.Fatal("unrelated output overwritten")
	}
}

func TestTrainAutomaticSeedsSharedSnapshotAndErrors(t *testing.T) {
	shared := filepath.Join(t.TempDir(), "snapshot")
	saveSnapshot(t, shared, loopSnapshot())
	workdir := newWorkdir(t, strings.Replace(trainingYAML, "seeds: [0, 1]", "runs: 2", 1))
	if err := runTrain(context.Background(), []string{"--workdir", workdir, "--snapshot", filepath.Join(shared, "osm.json")}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		ResolvedSeeds []uint64 `json:"resolved_seeds"`
		Automatic     bool     `json:"automatic_seeds"`
	}
	if err := json.Unmarshal(readFile(t, filepath.Join(workdir, "output", "training", "metadata.json")), &metadata); err != nil {
		t.Fatal(err)
	}
	if !metadata.Automatic || len(metadata.ResolvedSeeds) != 2 || metadata.ResolvedSeeds[0] == metadata.ResolvedSeeds[1] {
		t.Fatalf("metadata=%+v", metadata)
	}
	for _, args := range [][]string{nil, {"--workdir", workdir, "extra"}, {"--unknown"}, {"--workdir", workdir, "--snapshot", "missing.json"}} {
		if err := runTrain(context.Background(), args, io.Discard, io.Discard); err == nil {
			t.Fatalf("expected error for %v", args)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runTrain(ctx, []string{"--workdir", workdir}, io.Discard, io.Discard); err == nil {
		t.Fatal("cancelled command accepted")
	}
	far := newWorkdir(t, strings.Replace(trainingYAML, "latitude: 0", "latitude: 10", 1))
	saveSnapshot(t, filepath.Join(far, "output"), loopSnapshot())
	if err := runTrain(context.Background(), []string{"--workdir", far}, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "max_snap_distance_m") {
		t.Fatalf("snap error=%v", err)
	}
}

func TestFailedTrainingRetainsPreviousCompletedResult(t *testing.T) {
	workdir := newWorkdir(t, trainingYAML)
	cfg, err := config.Load(workdir)
	if err != nil {
		t.Fatal(err)
	}
	graph := osm.NewGraph()
	if err := graph.LoadOSM(loopSnapshot()); err != nil {
		t.Fatal(err)
	}
	prepared, err := qlearning.NewPreparedGraph(graph, 0, qlearning.DefaultRewardConfig())
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(workdir, "output")
	target := filepath.Join(output, "training")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "previous"), []byte("complete"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Force a worker validation failure after the running metadata was saved.
	cfg.QLearning.Environment.MaxSteps = 0
	err = executeTraining(context.Background(), io.Discard, prepared, cfg, []uint64{0}, output, map[string]any{"status": "running", "resolved_seeds": []uint64{0}})
	if err == nil {
		t.Fatal("expected training failure")
	}
	if string(readFile(t, filepath.Join(target, "previous"))) != "complete" {
		t.Fatal("failed attempt replaced previous results")
	}
}

func TestFailedAttemptStatusRecordsResolvedSeed(t *testing.T) {
	workdir := newWorkdir(t, trainingYAML)
	cfg, err := config.Load(workdir)
	if err != nil {
		t.Fatal(err)
	}
	graph := osm.NewGraph()
	if err := graph.LoadOSM(loopSnapshot()); err != nil {
		t.Fatal(err)
	}
	prepared, err := qlearning.NewPreparedGraph(graph, 0, qlearning.DefaultRewardConfig())
	if err != nil {
		t.Fatal(err)
	}
	cfg.QLearning.Environment.MaxSteps = 0
	output := filepath.Join(workdir, "output")
	if err := executeTraining(context.Background(), io.Discard, prepared, cfg, []uint64{42}, output, map[string]any{"status": "running", "resolved_seeds": []uint64{42}}); err == nil {
		t.Fatal("expected failure")
	}
	var marker struct {
		Status        string
		ResolvedSeeds []uint64 `json:"resolved_seeds"`
		Error         string
	}
	if err := json.Unmarshal(readFile(t, filepath.Join(output, "training-run.json")), &marker); err != nil {
		t.Fatal(err)
	}
	if marker.Status != "failed" || len(marker.ResolvedSeeds) != 1 || marker.ResolvedSeeds[0] != 42 || marker.Error == "" {
		t.Fatalf("marker=%+v", marker)
	}
}
