package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
)

func TestRunWorkflowUsesCurrentMapAndOneSnapshot(t *testing.T) {
	workdir := newWorkdir(t, trainingYAML)
	output := filepath.Join(workdir, "output")
	saveSnapshot(t, output, osm.OSMData{Version: 0.6, Generator: "stale", Elements: []osm.Element{}})
	fetcher := &fakeFetcher{data: loopSnapshot()}
	args := []string{"run", "--workdir", workdir}
	for i := range 4 {
		var out, logs bytes.Buffer
		runArgs := append([]string{}, args...)
		if i == 2 {
			runArgs = append(runArgs, "--force", "--debug")
		}
		if i == 3 {
			content := strings.Replace(trainingYAML, "latitude: 0", "latitude: 0.00001", 1)
			if err := os.WriteFile(filepath.Join(workdir, "config.yaml"), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if err := run(context.Background(), runArgs, &out, &logs, fetcher); err != nil {
			t.Fatal(err)
		}
		wantCalls := []int{1, 1, 2, 3}[i]
		if fetcher.calls != wantCalls {
			t.Fatalf("iteration %d: requests=%d, want %d", i, fetcher.calls, wantCalls)
		}
		if i == 3 && fetcher.params.Lat != 0.00001 {
			t.Fatalf("stale map parameters: %+v", fetcher.params)
		}
		hash, err := fileSHA256(filepath.Join(output, "osm.json"))
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"download.json", "analysis.json", "training-run.json"} {
			var meta struct {
				Status         string
				SnapshotSHA256 string `json:"snapshot_sha256"`
			}
			if err := json.Unmarshal(readFile(t, filepath.Join(output, name)), &meta); err != nil {
				t.Fatal(err)
			}
			if meta.Status != "completed" || meta.SnapshotSHA256 != hash {
				t.Fatalf("%s: %+v", name, meta)
			}
		}
		for _, name := range []string{"nodes.csv", "edges.csv", "road-analysis.html", "training/routes.html"} {
			if _, err := os.Stat(filepath.Join(output, name)); err != nil {
				t.Fatal(err)
			}
		}
		if !strings.Contains(out.String(), "Summary: greedy loops") || !strings.Contains(logs.String(), "Workflow completed") {
			t.Fatalf("missing summaries: %s %s", out.String(), logs.String())
		}
		if (i == 2) != strings.Contains(logs.String(), `"level":"DEBUG"`) {
			t.Fatal("debug flag was not forwarded")
		}
	}
}

func TestRunWorkflowSelectedProfile(t *testing.T) {
	content := strings.Replace(trainingYAML, "    episodes: 12\n", "    profiles:\n      - name: first\n        episodes: 2\n      - name: second\n        episodes: 2\n", 1)
	workdir := newWorkdir(t, content)
	err := run(context.Background(), []string{"run", "--workdir", workdir, "--profile", "second"}, io.Discard, io.Discard, &fakeFetcher{data: loopSnapshot()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(workdir, "output/training/second/routes.html")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(workdir, "output/training/first")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unselected profile ran: %v", err)
	}
}

func TestRunWorkflowStopsAfterFailure(t *testing.T) {
	for _, stage := range []string{"download", "analyze", "train"} {
		t.Run(stage, func(t *testing.T) {
			workdir := newWorkdir(t, trainingYAML)
			output := filepath.Join(workdir, "output")
			saveSnapshot(t, output, loopSnapshot())
			for _, name := range []string{"nodes.csv", "training-run.json"} {
				if err := os.WriteFile(filepath.Join(output, name), []byte("previous"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			fetcher := &fakeFetcher{data: loopSnapshot()}
			switch stage {
			case "download":
				fetcher.err = errors.New("offline")
			case "analyze":
				fetcher.data.Elements = []osm.Element{{Type: osm.TypeWay, ID: 100, Nodes: []int{8, 9}}}
			case "train":
				fetcher.data.Elements[0].Lat = 1
			}
			var logs bytes.Buffer
			err := run(context.Background(), []string{"run", "--workdir", workdir}, io.Discard, &logs, fetcher)
			if err == nil || !strings.Contains(err.Error(), stage+" stage failed") {
				t.Fatalf("error=%v", err)
			}
			if stage == "download" && strings.Contains(logs.String(), `"command":"analyze"`) {
				t.Fatal("analysis ran after failed download")
			}
			if stage == "analyze" && strings.Contains(logs.String(), `"command":"train"`) {
				t.Fatal("training ran after failed analysis")
			}
			if string(readFile(t, filepath.Join(output, "training-run.json"))) != "previous" {
				t.Fatal("failure replaced previous training")
			}
			if stage != "train" && string(readFile(t, filepath.Join(output, "nodes.csv"))) != "previous" {
				t.Fatal("failure replaced previous analysis")
			}
		})
	}
}

func TestRunWorkflowValidationAndCancellation(t *testing.T) {
	workdir := newWorkdir(t, trainingYAML)
	for _, args := range [][]string{
		{"run"}, {"run", "--unknown"}, {"run", "--workdir", workdir, "extra"},
		{"run", "--workdir", workdir, "--snapshot", "old.json"},
		{"run", "--workdir", workdir, "--profile", "missing"},
		{"run", "--workdir", newWorkdir(t, testConfig)},
	} {
		fetcher := &fakeFetcher{}
		if err := run(context.Background(), args, io.Discard, io.Discard, fetcher); err == nil || fetcher.calls != 0 {
			t.Fatalf("args=%v calls=%d error=%v", args, fetcher.calls, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fetcher := &fakeFetcher{}
	if err := run(ctx, []string{"run", "--workdir", workdir}, io.Discard, io.Discard, fetcher); !errors.Is(err, context.Canceled) || fetcher.calls != 0 {
		t.Fatalf("calls=%d error=%v", fetcher.calls, err)
	}
	var out bytes.Buffer
	if err := run(context.Background(), []string{"run", "--help"}, &out, io.Discard, fetcher); err != nil || !strings.Contains(out.String(), "cli run --workdir") || fetcher.calls != 0 {
		t.Fatalf("help error=%v", err)
	}
}
