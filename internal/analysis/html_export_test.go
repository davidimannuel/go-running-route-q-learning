package analysis

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/qlearning"
)

func TestPortableMapPreservesSavedDataAndLargeSeeds(t *testing.T) {
	graph := analysisTestGraph(t)
	edge := &graph.OutgoingEdges(1)[0]
	trials := []RouteTrial{
		{Metrics: qlearning.Metrics{Scenario: "C", Seed: 18446744073709551615, TotalDistanceM: 123, TotalReward: -12, TerminationReason: qlearning.NoAvailableAction}, Steps: []qlearning.StepResult{{Transition: qlearning.Transition{ActionTaken: true, Edge: edge}, Reward: -12}}},
		{Metrics: qlearning.Metrics{Scenario: "C", Seed: 0, TotalDistanceM: 5000, IsLoop: true}},
	}
	var original bytes.Buffer
	if err := RoutesToHTML(&original, graph, 1, 0, 0, 0, trials, qlearning.DefaultRewardConfig().Road); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"standard", "advanced"} {
		var exported bytes.Buffer
		opts := HTMLOptions{MapMode: mode, IndexURL: "../../index.html", SeedSource: SeedSource(true), QSource: "../../../training/profile/routes.html"}
		summary, err := ReexportRoutesToHTML(&exported, strings.NewReader(original.String()), opts)
		if err != nil {
			t.Fatal(err)
		}
		if summary.Trials[0].ID != "18446744073709551615" || !summary.Trials[1].Representative {
			t.Fatalf("summary=%+v", summary)
		}
		// Re-read from a new reader because the exporter consumes its input.
		var before, after routeMapData
		if err := readEmbeddedHTML(strings.NewReader(original.String()), "const routeData = ", ";\n", &before); err != nil {
			t.Fatal(err)
		}
		if err := readEmbeddedHTML(strings.NewReader(exported.String()), "const routeData = ", ";\n", &after); err != nil {
			t.Fatal(err)
		}
		after.HTML = before.HTML
		if !reflect.DeepEqual(before, after) {
			t.Fatal("export changed graph, routes, reward, seed, or metrics")
		}
		for _, want := range []string{`id="seed-prev"`, `id="seed-next"`, `id="seed-id"`, `href="../../index.html"`, "Show graph nodes", "Comfort score", "Node access", "Distance error penalty", "searchParams.get('seed')"} {
			if !strings.Contains(exported.String(), want) {
				t.Fatalf("missing feature %q", want)
			}
		}
		for _, q := range []string{"fetch(", "qView", "createQValueInspector", `id="q-node-tools"`, "'/q-values'"} {
			if strings.Contains(exported.String(), q) != (mode == "advanced") {
				t.Fatalf("mode %s: Q dependency %q", mode, q)
			}
		}
	}
}

func TestAnalysisReexportAndUnsupportedHTML(t *testing.T) {
	var original, exported bytes.Buffer
	if err := MapToHTML(&original, analysisTestGraph(t), 0, 0, qlearning.DefaultRewardConfig().Road); err != nil {
		t.Fatal(err)
	}
	if err := ReexportAnalysisToHTML(&exported, strings.NewReader(original.String()), HTMLOptions{MapMode: "standard", IndexURL: "index.html"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(exported.String(), `href="index.html"`) || strings.Contains(exported.String(), "fetch(") {
		t.Fatal("analysis is not portable")
	}
	for _, bad := range []string{"<html>old format</html>", "const routeData = {bad};\n", `const routeData = {};` + "\n"} {
		if _, err := ReexportRoutesToHTML(&bytes.Buffer{}, strings.NewReader(bad), HTMLOptions{}); err == nil {
			t.Fatal("unsupported HTML accepted")
		}
	}
}
