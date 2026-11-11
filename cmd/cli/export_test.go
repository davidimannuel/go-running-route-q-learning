package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/analysis"
	"github.com/davidimannuel/go-running-route-q-learning/internal/htmlserver"
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

func TestExportSavedResultsPortableAndNoSourceChanges(t *testing.T) {
	for _, mode := range []string{"standard", "advanced"} {
		t.Run(mode, func(t *testing.T) {
			workdir := savedExportWorkdir(t, mode)
			original := filepath.Join(workdir, "output", "training", "b-saved", "routes.html")
			before := string(readFile(t, original))
			var out bytes.Buffer
			if err := run(context.Background(), []string{"export", "--workdir", workdir}, &out, &out, nil); err != nil {
				t.Fatal(err)
			}
			site := filepath.Join(workdir, "output", "site")
			route := string(readFile(t, filepath.Join(site, "training", "b-saved", "routes.html")))
			if strings.Contains(route, "fetch(") != (mode == "advanced") {
				t.Fatalf("wrong dependency for %s", mode)
			}
			for _, want := range []string{`href="../../index.html"`, "18446744073709551615", "Random seed generated once", `id="seed-next"`} {
				if !strings.Contains(route, want) {
					t.Fatalf("missing %q", want)
				}
			}
			if string(readFile(t, original)) != before {
				t.Fatal("original map changed")
			}
			report := string(readFile(t, filepath.Join(site, "training", "b-saved", "report.html")))
			if !strings.Contains(report, "150000") || !strings.Contains(report, "18446744073709551615") || strings.Contains(report, `href="/"`) {
				t.Fatal("report did not preserve saved config and portable links")
			}
			for _, forbidden := range []string{"/q-values", "/details?", "/files/"} {
				if strings.Contains(string(readFile(t, filepath.Join(site, "index.html"))), forbidden) {
					t.Fatalf("static index references %s", forbidden)
				}
			}
			if strings.Contains(string(readFile(t, filepath.Join(site, "analysis.html"))), "old saved scoring") == false {
				t.Fatal("analysis metadata recomputed")
			}
			if _, err := os.Stat(filepath.Join(site, "training", "b-saved", "qtable.csv")); !os.IsNotExist(err) {
				t.Fatal("raw Q-table published")
			}
			// Standard pages work under a generic static server and a subpath.
			h := http.StripPrefix("/catalog/", http.FileServer(http.Dir(site)))
			for _, name := range []string{"", "road-analysis.html", "analysis.html", "training/b-saved/routes.html", "training/b-saved/report.html", "export.json"} {
				w := httptest.NewRecorder()
				h.ServeHTTP(w, httptest.NewRequest("GET", "/catalog/"+name, nil))
				if w.Code != 200 {
					t.Fatalf("static %s: %d", name, w.Code)
				}
			}
			if mode == "advanced" {
				if !strings.Contains(route, `../../../training/b-saved/routes.html`) {
					t.Fatal("advanced map lost original Q source")
				}
				// Resolve the actual exported-page source URL to its original result.
				h, err := htmlserver.New(filepath.Join(workdir, "output"))
				if err != nil {
					t.Fatal(err)
				}
				defer h.Close()
				w := httptest.NewRecorder()
				h.ServeHTTP(w, httptest.NewRequest("GET", "/files/site/training/b-saved/routes.html", nil))
				if w.Code != 200 {
					t.Fatal("advanced map not served")
				}
			}
		})
	}
}

func TestExportFailureKeepsPreviousSiteAndProfileSelection(t *testing.T) {
	workdir := savedExportWorkdir(t, "standard")
	args := []string{"export", "--workdir", workdir, "--profile", "b-saved"}
	if err := run(context.Background(), args, io.Discard, io.Discard, nil); err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(workdir, "output", "site", "index.html")
	before := string(readFile(t, index))
	if strings.Contains(before, "b-other") || strings.Contains(before, "Road analysis") {
		t.Fatal("profile selection ignored")
	}
	meta := filepath.Join(workdir, "output", "training", "b-saved", "metadata.json")
	if err := os.WriteFile(meta, []byte(`{"status":"completed","config":{"scenario":"C"},"resolved_seeds":[18446744073709551615,0]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), args, io.Discard, io.Discard, nil); err == nil {
		t.Fatal("mismatched result accepted")
	}
	if string(readFile(t, index)) != before {
		t.Fatal("failed export damaged previous site")
	}
	for _, profile := range []string{"../escape", "/absolute", "missing"} {
		if err := run(context.Background(), []string{"export", "--workdir", workdir, "--profile", profile}, io.Discard, io.Discard, nil); err == nil {
			t.Fatal(fmt.Sprintf("invalid profile %s accepted", profile))
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := run(ctx, args, io.Discard, io.Discard, nil); err == nil {
		t.Fatal("cancellation ignored")
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
	if _, err := os.Stat(filepath.Join(workdir, "output", "site", "training", "routes.html")); err != nil {
		t.Fatal(err)
	}
}
