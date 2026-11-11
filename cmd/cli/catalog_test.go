package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/analysis"
	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
	"github.com/davidimannuel/go-running-route-q-learning/internal/qlearning"
)

const catalogSeed = "18446744073709551615"

func catalogFixture(t *testing.T) (string, string) {
	t.Helper()
	workdir := newWorkdir(t, testConfig+"export:\n  map_mode: advanced\nqlearning:\n  reward:\n    success_bonus: 999\n")
	graph := osm.NewGraph()
	if err := graph.LoadOSM(osm.OSMData{Elements: []osm.Element{
		{Type: osm.TypeNode, ID: 1, Lat: -7.281518, Lon: 112.710178},
		{Type: osm.TypeNode, ID: 2, Lat: -7.280518, Lon: 112.710178},
		{Type: osm.TypeNode, ID: 3, Lat: -7.280518, Lon: 112.711178},
		{Type: osm.TypeWay, ID: 5, Nodes: []int{1, 2, 3, 1}, Tags: osm.Tags{"highway": "residential", "surface": "asphalt", "width": "4"}},
	}}); err != nil {
		t.Fatal(err)
	}
	var steps []qlearning.StepResult
	for i, pair := range [][2]int{{1, 2}, {2, 3}, {3, 1}} {
		edges := graph.OutgoingEdges(pair[0])
		for j := range edges {
			if edges[j].To == pair[1] {
				length := 2000.0
				if i == 2 {
					length = 1000
				}
				edges[j].LengthM = length
				steps = append(steps, qlearning.StepResult{Transition: qlearning.Transition{ActionTaken: true, Edge: &edges[j], Step: i + 1}, Reward: length / 100})
			}
		}
	}
	metrics := qlearning.Metrics{Scenario: "B", Seed: ^uint64(0), IsLoop: true, TotalDistanceM: 5000, TotalReward: 50, MeanComfort: 1, TerminationReason: qlearning.Success, Steps: 3}
	trials := []analysis.RouteTrial{{Metrics: metrics, Steps: steps}, {Metrics: qlearning.Metrics{Scenario: "B", Seed: 0, TerminationReason: qlearning.NoAvailableAction}}}
	directory := filepath.Join(workdir, "output", "training", "b-saved")
	if err := writeOutput(context.Background(), filepath.Join(directory, "routes.html"), func(w io.Writer) error {
		return analysis.RoutesToHTML(w, graph, 1, -7.281518, 112.710178, 0, trials, qlearning.DefaultRewardConfig().Road)
	}); err != nil {
		t.Fatal(err)
	}
	cfg := qlearning.DefaultConfig()
	cfg.Scenario = "B"
	cfg.Environment.DistanceTolerance = 0.2
	metadata := map[string]any{"status": "completed", "completed_at": "2026-11-12T00:00:00Z", "training_profile": "b-saved", "config": cfg, "automatic_seeds": true,
		"resolved_seeds": []uint64{^uint64(0), 0}, "start_node": 1, "nodes": len(graph.Nodes), "directed_edges": graph.EdgeCount(), "snapshot_sha256": "synthetic-fixture",
		"evaluations": []map[string]any{evaluationMetadata(metrics), evaluationMetadata(trials[1].Metrics)}}
	if err := writeJSON(context.Background(), filepath.Join(directory, "metadata.json"), metadata); err != nil {
		t.Fatal(err)
	}
	return workdir, filepath.Join(t.TempDir(), "web-catalog")
}

func catalogArgs(workdir, directory, seed string) []string {
	return []string{"add", "--workdir", workdir, "--profile", "b-saved", "--seed", seed, "--dir", directory}
}

func TestCatalogOneSeedStandardKeepsSourceAndOtherRoutes(t *testing.T) {
	workdir, directory := catalogFixture(t)
	source := filepath.Join(workdir, "output", "training", "b-saved")
	beforeMap, beforeMetadata := string(readFile(t, filepath.Join(source, "routes.html"))), string(readFile(t, filepath.Join(source, "metadata.json")))
	var out bytes.Buffer
	note := `Near campus <script>alert("note")</script>`
	if err := runCatalog(context.Background(), catalogArgs(workdir, directory, catalogSeed), strings.NewReader("campus\n"+note+"\n"), &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Route ID: ") || !strings.Contains(out.String(), "Route description: ") || strings.Contains(out.String(), "Route name") {
		t.Fatal("wrong prompts")
	}
	mapPath := filepath.Join(directory, "routes", "campus", "index.html")
	html := string(readFile(t, mapPath))
	for _, absent := range []string{"fetch(", "qView", "createQValueInspector", "'/q-values'", "Route name"} {
		if strings.Contains(html, absent) {
			t.Fatalf("catalog contains %s", absent)
		}
	}
	var data struct {
		HTML   analysis.HTMLOptions
		Trials []struct{ Seed string }
	}
	start := strings.Index(html, "const routeData = ") + len("const routeData = ")
	end := strings.Index(html[start:], ";\n") + start
	if err := json.Unmarshal([]byte(html[start:end]), &data); err != nil {
		t.Fatal(err)
	}
	if data.HTML.MapMode != "standard" || !data.HTML.Catalog || data.HTML.Description != note || len(data.Trials) != 1 || data.Trials[0].Seed != catalogSeed {
		t.Fatalf("data=%+v", data)
	}
	for _, want := range []string{`href="../../index.html"`, `href="details.html"`, "Comfort score", "Node access", "Direction arrows", "Highlight repeated roads"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %s", want)
		}
	}
	index := string(readFile(t, filepath.Join(directory, "index.html")))
	if strings.Contains(index, note) || !strings.Contains(index, "&lt;script&gt;") || !strings.Contains(index, "routes/campus/index.html") {
		t.Fatal("notes escaping or relative link missing")
	}
	report := string(readFile(t, filepath.Join(directory, "routes", "campus", "details.html")))
	if !strings.Contains(report, `href="index.html"`) || !strings.Contains(report, catalogSeed) || !strings.Contains(report, "Selected seed") || strings.Contains(report, "999") {
		t.Fatal("report did not use source config")
	}
	// A duplicate ID prompts again; the first route stays byte-for-byte unchanged.
	before := string(readFile(t, mapPath))
	if err := runCatalog(context.Background(), catalogArgs(workdir, directory, catalogSeed), strings.NewReader("campus\nsecond-route\nAnother note\n"), io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if string(readFile(t, mapPath)) != before {
		t.Fatal("existing route changed")
	}
	manifest, exists, err := loadCatalog(directory)
	if err != nil || !exists || len(manifest.Routes) != 2 {
		t.Fatalf("manifest=%+v exists=%v err=%v", manifest, exists, err)
	}
	if beforeMap != string(readFile(t, filepath.Join(source, "routes.html"))) || beforeMetadata != string(readFile(t, filepath.Join(source, "metadata.json"))) {
		t.Fatal("source result changed")
	}
	if _, err := os.Stat(filepath.Join(directory, ".nojekyll")); err != nil {
		t.Fatal(err)
	}
	_ = filepath.WalkDir(directory, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			t.Fatal(err)
		}
		if !d.IsDir() && (strings.HasSuffix(path, ".csv") || strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, "osm.json")) {
			t.Fatalf("raw input published: %s", path)
		}
		return nil
	})
}

func TestCatalogFailuresDoNotPublish(t *testing.T) {
	workdir, directory := catalogFixture(t)
	for _, seed := range []string{"0", "1", "-1", "", "01"} {
		if err := runCatalog(context.Background(), catalogArgs(workdir, directory, seed), strings.NewReader("unused\nnote\n"), io.Discard, io.Discard); err == nil {
			t.Fatalf("invalid or failed seed %s accepted", seed)
		}
	}
	if err := runCatalog(context.Background(), catalogArgs(workdir, directory, catalogSeed), strings.NewReader(""), io.Discard, io.Discard); err == nil {
		t.Fatal("EOF published a route")
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatal("failed addition created catalog")
	}
	metadataPath := filepath.Join(workdir, "output", "training", "b-saved", "metadata.json")
	broken := strings.Replace(string(readFile(t, metadataPath)), `"total_distance_m": 5000`, `"total_distance_m": 5001`, 1)
	if err := os.WriteFile(metadataPath, []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runCatalog(context.Background(), catalogArgs(workdir, directory, catalogSeed), strings.NewReader("unused\nnote\n"), io.Discard, io.Discard); err == nil {
		t.Fatal("mixed metadata accepted")
	}
}

func TestCatalogInputValidationAndCancellation(t *testing.T) {
	workdir, directory := catalogFixture(t)
	var out bytes.Buffer
	if err := runCatalog(context.Background(), catalogArgs(workdir, directory, catalogSeed), strings.NewReader("../escape\n\ngood-id\n\nA route note\n"), &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "unused ID") || !strings.Contains(out.String(), "Enter a description") {
		t.Fatal("input validation not explained")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runCatalog(ctx, catalogArgs(workdir, directory, catalogSeed), strings.NewReader("new\nnote\n"), io.Discard, io.Discard); err == nil {
		t.Fatal("cancellation ignored")
	}
	ctx, cancel = context.WithCancel(context.Background())
	reader, writer := io.Pipe()
	lines := catalogInputLines(ctx, reader)
	cancel()
	if _, _, err := promptCatalogDetails(ctx, lines, io.Discard, catalogManifest{}); err == nil {
		t.Fatal("waiting prompt ignored cancellation")
	}
	writer.Close()
	reader.Close()
}
