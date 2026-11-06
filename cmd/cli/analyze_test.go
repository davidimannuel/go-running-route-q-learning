package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
)

func saveSnapshot(t *testing.T, output string, data osm.OSMData) []byte {
	t.Helper()
	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "osm.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	return body
}

func analyzeData() osm.OSMData {
	return osm.OSMData{Version: 0.6, Elements: []osm.Element{
		{Type: osm.TypeWay, ID: 100, Nodes: []int{1, 2}, Tags: osm.Tags{"width": "3;4", "access": "permissive"}},
		{Type: osm.TypeWay, ID: 101, Nodes: []int{1, 2}},
		{Type: osm.TypeNode, ID: 2, Lat: 0, Lon: 0.001},
		{Type: osm.TypeNode, ID: 1, Lat: 0, Lon: 0},
	}}
}

func TestAnalyze(t *testing.T) {
	for _, tc := range []struct {
		name, suffix, directory string
		debug                   bool
	}{
		{name: "default", directory: "output"},
		{name: "custom output and debug", suffix: "output_dir: results/graph\n", directory: "results/graph", debug: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workdir := newWorkdir(t, testConfig+tc.suffix)
			output := filepath.Join(workdir, tc.directory)
			original := saveSnapshot(t, output, analyzeData())
			other := filepath.Join(output, "training.csv")
			if err := os.WriteFile(other, []byte("keep"), 0o600); err != nil {
				t.Fatal(err)
			}
			fetcher := &fakeFetcher{err: errors.New("analyze must not fetch")}
			args := []string{"analyze", "--workdir", workdir}
			if tc.debug {
				args = append(args, "--debug")
			}
			var stdout, logs bytes.Buffer
			if err := run(context.Background(), args, &stdout, &logs, fetcher); err != nil {
				t.Fatal(err)
			}
			if stdout.Len() != 0 || fetcher.calls != 0 {
				t.Fatalf("stdout=%s; requests=%d", stdout.String(), fetcher.calls)
			}
			for _, message := range []string{"Analyzing OSM snapshot", "Graph built", "Saved graph CSV", "Saved road analysis map", "Analysis completed", `"nodes":2`, `"directed_edges":4`} {
				if !strings.Contains(logs.String(), message) {
					t.Fatalf("missing %q in logs: %s", message, logs.String())
				}
			}
			if strings.Contains(logs.String(), `"level":"DEBUG"`) != tc.debug {
				t.Fatalf("unexpected log level: %s", logs.String())
			}
			nodes, err := csv.NewReader(bytes.NewReader(readFile(t, filepath.Join(output, "nodes.csv")))).ReadAll()
			if err != nil || len(nodes) != 3 || nodes[1][0] != "1" || nodes[2][0] != "2" {
				t.Fatalf("nodes=%v, error=%v", nodes, err)
			}
			edges, err := csv.NewReader(bytes.NewReader(readFile(t, filepath.Join(output, "edges.csv")))).ReadAll()
			if err != nil || len(edges) != 5 || edges[1][3] != "100" || edges[1][8] != "3;4" || edges[1][9] != "permissive" || edges[2][8] != "" {
				t.Fatalf("edges=%v, error=%v", edges, err)
			}
			mapHTML := string(readFile(t, filepath.Join(output, "road-analysis.html")))
			if !strings.Contains(mapHTML, `"center":[-7.282187,112.709282]`) || !strings.Contains(mapHTML, `"comfort_score":-0.5`) || !strings.Contains(mapHTML, `"key":1`) {
				t.Fatal("map is missing input location, scores, or parallel edge identity")
			}
			if !bytes.Equal(original, readFile(t, filepath.Join(output, "osm.json"))) {
				t.Fatal("analysis modified the snapshot")
			}
			// A smaller replacement snapshot must overwrite old tables, including stale rows.
			saveSnapshot(t, output, osm.OSMData{Version: 0.6, Elements: []osm.Element{{Type: osm.TypeNode, ID: 99}}})
			if err := run(context.Background(), args, &stdout, &logs, fetcher); err != nil {
				t.Fatal(err)
			}
			nodes, err = csv.NewReader(bytes.NewReader(readFile(t, filepath.Join(output, "nodes.csv")))).ReadAll()
			if err != nil || len(nodes) != 2 || nodes[1][0] != "99" {
				t.Fatalf("CSV was not overwritten: %v, error=%v", nodes, err)
			}
			edges, err = csv.NewReader(bytes.NewReader(readFile(t, filepath.Join(output, "edges.csv")))).ReadAll()
			if err != nil || len(edges) != 1 || string(readFile(t, other)) != "keep" {
				t.Fatalf("edges=%v, error=%v", edges, err)
			}
			mapHTML = string(readFile(t, filepath.Join(output, "road-analysis.html")))
			if !strings.Contains(mapHTML, `"id":"99"`) || !strings.Contains(mapHTML, `"edges":[]`) || strings.Contains(mapHTML, `"way_id":"100"`) {
				t.Fatal("map was not replaced with the current graph")
			}
			entries, err := os.ReadDir(output)
			if err != nil || len(entries) != 5 {
				t.Fatalf("temporary output was not cleaned up: %v, error=%v", entries, err)
			}
		})
	}
}

func TestAnalyzeRejectsInvalidSnapshots(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"missing", "", "run download first"},
		{"malformed", "not JSON", "decode"},
		{"trailing JSON", `{"version":0.6,"elements":[]} {}`, "decode"},
		{"null elements", `{"version":0.6,"elements":null}`, "elements"},
		{"missing version", `{"elements":[]}`, "version"},
		{"remark", `{"version":0.6,"elements":[],"remark":"runtime error"}`, "server remark"},
		{"missing node", `{"version":0.6,"elements":[{"type":"way","id":100,"nodes":[1,2]}]}`, "missing node"},
		{"invalid coordinates", `{"version":0.6,"elements":[{"type":"node","id":1,"lat":91},{"type":"node","id":2},{"type":"way","id":100,"nodes":[1,2]}]}`, "latitude"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workdir := newWorkdir(t, testConfig)
			output := filepath.Join(workdir, "output")
			if err := os.MkdirAll(output, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"nodes.csv", "edges.csv", "road-analysis.html"} {
				if err := os.WriteFile(filepath.Join(output, name), []byte("previous export"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.body != "" {
				if err := os.WriteFile(filepath.Join(output, "osm.json"), []byte(tc.body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			fetcher := &fakeFetcher{}
			err := run(context.Background(), []string{"analyze", "--workdir", workdir}, io.Discard, io.Discard, fetcher)
			if err == nil || !strings.Contains(err.Error(), tc.want) || fetcher.calls != 0 {
				t.Fatalf("requests=%d, error=%v", fetcher.calls, err)
			}
			for _, name := range []string{"nodes.csv", "edges.csv", "road-analysis.html"} {
				if string(readFile(t, filepath.Join(output, name))) != "previous export" {
					t.Fatalf("invalid snapshot changed %s", name)
				}
			}
		})
	}
}

func TestAnalyzeValidationAndCancellation(t *testing.T) {
	for _, args := range [][]string{{"analyze"}, {"analyze", "--unknown"}, {"analyze", "--force"}, {"analyze", "--workdir", t.TempDir(), "extra"}} {
		if err := run(context.Background(), args, io.Discard, io.Discard, &fakeFetcher{}); err == nil {
			t.Fatalf("expected an error for %v", args)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := run(ctx, []string{"analyze", "--workdir", t.TempDir()}, io.Discard, io.Discard, &fakeFetcher{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation was lost: %v", err)
	}
}

func TestWriteOutputPreservesPreviousFile(t *testing.T) {
	for _, stage := range []string{"writer error", "cancellation"} {
		t.Run(stage, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "nodes.csv")
			if err := os.WriteFile(path, []byte("previous export"), 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			sentinel := errors.New("writer failed")
			err := writeOutput(ctx, path, func(w io.Writer) error {
				if _, err := io.WriteString(w, "partial export"); err != nil {
					return err
				}
				if stage == "writer error" {
					return sentinel
				}
				cancel()
				return nil
			})
			want := sentinel
			if stage == "cancellation" {
				want = context.Canceled
			}
			if !errors.Is(err, want) || string(readFile(t, path)) != "previous export" {
				t.Fatalf("error=%v", err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatalf("temporary file remained: %v, %v", entries, err)
			}
		})
	}
}
