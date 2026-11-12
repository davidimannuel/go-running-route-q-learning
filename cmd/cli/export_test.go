package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/analysis"
	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
	"github.com/davidimannuel/go-running-route-q-learning/internal/qlearning"
)

func savedExportWorkdir(t *testing.T, mode string) string {
	t.Helper()
	workdir := newWorkdir(t, testConfig+"export:\n  map_mode: "+mode+"\n")
	graph := osm.NewGraph()
	if err := graph.LoadOSM(osm.OSMData{Elements: []osm.Element{
		{Type: osm.TypeNode, ID: 1, Lat: -7.281518, Lon: 112.710178},
		{Type: osm.TypeNode, ID: 2, Lat: -7.280518, Lon: 112.710178, Tags: osm.Tags{"access": "no", "barrier": "gate"}},
		{Type: osm.TypeWay, ID: 5, Nodes: []int{1, 2}, Tags: osm.Tags{"highway": "residential", "width": "4"}},
	}}); err != nil {
		t.Fatal(err)
	}
	trials := []analysis.RouteTrial{
		{Metrics: qlearning.Metrics{Scenario: "B", Seed: 18446744073709551615, TotalDistanceM: 123, TerminationReason: qlearning.NoAvailableAction}},
		{Metrics: qlearning.Metrics{Scenario: "B", Seed: 0, IsLoop: true, TotalDistanceM: 5000}},
	}
	for _, name := range []string{"b-saved", "b-other"} {
		directory := filepath.Join(workdir, "output", "training", name)
		if err := writeOutput(context.Background(), filepath.Join(directory, "routes.html"), func(w io.Writer) error {
			return analysis.RoutesToHTML(w, graph, 1, -7.281518, 112.710178, 0, trials, qlearning.DefaultRewardConfig().Road)
		}); err != nil {
			t.Fatal(err)
		}
		// The saved config is deliberately different from the current workdir.
		metadata := map[string]any{"status": "completed", "completed_at": "2026-11-11T01:00:00Z", "config": map[string]any{"scenario": "B", "training": map[string]any{"episodes": 150000}, "reward": map[string]any{"revisit_penalty": -10}}, "resolved_seeds": []uint64{18446744073709551615, 0}, "automatic_seeds": true}
		if err := writeJSON(context.Background(), filepath.Join(directory, "metadata.json"), metadata); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "qtable.csv"), []byte("private test Q-table"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeOutput(context.Background(), filepath.Join(workdir, "output", "road-analysis.html"), func(w io.Writer) error {
		return analysis.MapToHTML(w, graph, -7.281518, 112.710178, qlearning.DefaultRewardConfig().Road)
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(context.Background(), filepath.Join(workdir, "output", "analysis.json"), map[string]any{"status": "completed", "completed_at": "2026-11-09T01:00:00Z", "road_score": "old saved scoring"}); err != nil {
		t.Fatal(err)
	}
	return workdir
}

// Every non-HTML research file must retain its exact recorded bytes.
func assertResearchUnchanged(t *testing.T, output string, before map[string][]byte) {
	t.Helper()
	for name, body := range before {
		if filepath.Ext(name) != ".html" && !bytes.Equal(readFile(t, filepath.Join(output, name)), body) {
			t.Fatalf("research data changed: %s", name)
		}
	}
}

func TestExportUpdatesOriginalHTMLAndPreservesResearch(t *testing.T) {
	for _, mode := range []string{"standard", "advanced"} {
		t.Run(mode, func(t *testing.T) {
			workdir := savedExportWorkdir(t, mode)
			output := filepath.Join(workdir, "output")
			before := treeFiles(t, output)
			var out bytes.Buffer
			args := []string{"export", "--workdir", workdir}
			for range 2 { // A refreshed map remains a valid source for the next export.
				if err := run(context.Background(), args, &out, &out, nil); err != nil {
					t.Fatal(err)
				}
			}
			route := string(readFile(t, filepath.Join(output, "training", "b-saved", "routes.html")))
			if strings.Contains(route, "fetch(") != (mode == "advanced") {
				t.Fatalf("wrong dependency for %s", mode)
			}
			for _, want := range []string{`href="/"`, "18446744073709551615", "Random seed generated once", `id="seed-next"`, `id="download-gpx"`} {
				if !strings.Contains(route, want) {
					t.Fatalf("missing %q", want)
				}
			}
			if strings.Contains(route, `../../../training/`) || !strings.Contains(route, `"QSource":""`) {
				t.Fatal("in-place map must use its own location for Q queries")
			}
			report := string(readFile(t, filepath.Join(output, "training", "b-saved", "report.html")))
			if !strings.Contains(report, "150000") || !strings.Contains(report, "18446744073709551615") {
				t.Fatal("report did not preserve saved config and uint64 seeds")
			}
			if !strings.Contains(string(readFile(t, filepath.Join(output, "analysis.html"))), "old saved scoring") {
				t.Fatal("analysis metadata recomputed")
			}
			for _, name := range []string{"site", "index.html"} {
				if _, err := os.Stat(filepath.Join(output, name)); !os.IsNotExist(err) {
					t.Fatalf("unexpected duplicate presentation: %s", name)
				}
			}
			assertResearchUnchanged(t, output, before)
		})
	}
}

func TestExportProfileSelectionAndFailurePreserveOtherResults(t *testing.T) {
	workdir := savedExportWorkdir(t, "standard")
	output := filepath.Join(workdir, "output")
	before := treeFiles(t, output)
	args := []string{"export", "--workdir", workdir, "--profile", "b-saved"}
	if err := run(context.Background(), args, io.Discard, io.Discard, nil); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"training/b-other/routes.html", "road-analysis.html"} {
		if !bytes.Equal(before[name], readFile(t, filepath.Join(output, name))) {
			t.Fatalf("unselected map changed: %s", name)
		}
	}
	assertResearchUnchanged(t, output, before)
	meta := filepath.Join(output, "training", "b-saved", "metadata.json")
	if err := os.WriteFile(meta, []byte(`{"status":"completed","config":{"scenario":"C"},"resolved_seeds":[18446744073709551615,0]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	unchanged := treeFiles(t, output)
	if err := run(context.Background(), args, io.Discard, io.Discard, nil); err == nil {
		t.Fatal("mismatched result accepted")
	}
	for name, body := range unchanged {
		if !bytes.Equal(readFile(t, filepath.Join(output, name)), body) {
			t.Fatalf("failed export changed %s", name)
		}
	}
	for _, profile := range []string{"../escape", "/absolute", "missing"} {
		if err := run(context.Background(), []string{"export", "--workdir", workdir, "--profile", profile}, io.Discard, io.Discard, nil); err == nil {
			t.Fatalf("invalid profile %s accepted", profile)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := run(ctx, args, io.Discard, io.Discard, nil); err == nil {
		t.Fatal("cancellation ignored")
	}
}

func legacySiteFixture(t *testing.T, workdir string) string {
	t.Helper()
	site := filepath.Join(workdir, "output", "site")
	for _, name := range []string{"index.html", "training/b-saved/routes.html", "training/b-saved/report.html"} {
		if err := writeOutput(context.Background(), filepath.Join(site, name), func(w io.Writer) error {
			_, err := io.WriteString(w, "old exported HTML")
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeJSON(context.Background(), filepath.Join(site, "export.json"), map[string]any{
		"schema_version": 1, "command": "export", "workdir": workdir,
		"sources": []map[string]string{{"file": "training/b-saved/routes.html"}},
	}); err != nil {
		t.Fatal(err)
	}
	return site
}

func TestExportCleansOnlyVerifiedLegacySite(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		t.Run(map[bool]string{false: "generated", true: "user-file"}[unknown], func(t *testing.T) {
			workdir := savedExportWorkdir(t, "standard")
			site := legacySiteFixture(t, workdir)
			if unknown {
				if err := os.WriteFile(filepath.Join(site, "notes.txt"), []byte("keep this"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			output := filepath.Join(workdir, "output")
			before := treeFiles(t, output)
			err := run(context.Background(), []string{"export", "--workdir", workdir}, io.Discard, io.Discard, nil)
			if unknown {
				if err == nil {
					t.Fatal("unrecognized site contents removed")
				}
				for name, body := range before {
					if !bytes.Equal(body, readFile(t, filepath.Join(output, name))) {
						t.Fatalf("failed cleanup changed %s", name)
					}
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(site); !os.IsNotExist(err) {
					t.Fatal("old duplicate site remains")
				}
			}
		})
	}
}

func TestExportPublicationRollsBackHTMLAndSite(t *testing.T) {
	workdir := savedExportWorkdir(t, "standard")
	output := filepath.Join(workdir, "output")
	legacySiteFixture(t, workdir)
	// A directory in place of a report forces publication to fail after earlier
	// files have been replaced. Both those files and the site must be restored.
	if err := os.Mkdir(filepath.Join(output, "training", "b-saved", "report.html"), 0o755); err != nil {
		t.Fatal(err)
	}
	before := treeFiles(t, output)
	if err := run(context.Background(), []string{"export", "--workdir", workdir}, io.Discard, io.Discard, nil); err == nil {
		t.Fatal("invalid destination accepted")
	}
	for name, body := range before {
		if !bytes.Equal(body, readFile(t, filepath.Join(output, name))) {
			t.Fatalf("rollback changed %s", name)
		}
	}
	if _, err := os.Stat(filepath.Join(output, "export.json")); !os.IsNotExist(err) {
		t.Fatal("failed export left a new manifest")
	}
}

func TestExportLegacySingleProfile(t *testing.T) {
	workdir := savedExportWorkdir(t, "standard")
	training := filepath.Join(workdir, "output", "training")
	moved := filepath.Join(workdir, "old-training")
	if err := os.Rename(training, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(moved, "b-saved"), training); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(training, "seed-0"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), []string{"export", "--workdir", workdir}, io.Discard, io.Discard, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(training, "report.html")); err != nil {
		t.Fatal(err)
	}
}
