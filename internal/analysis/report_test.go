package analysis

import (
	"bytes"
	"strings"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/qlearning"
)

func TestConfiguredScoresAndRecordedReport(t *testing.T) {
	g := analysisTestGraph(t)
	scores := qlearning.DefaultRewardConfig().Road
	scores.Highway.Priority = 2
	var csv, html bytes.Buffer
	if err := EdgeToCSV(&csv, g, scores); err != nil {
		t.Fatal(err)
	}
	if err := MapToHTML(&html, g, 0, 0, scores); err != nil {
		t.Fatal(err)
	}
	data := readMapData(t, html.String())
	found := false
	for _, edge := range data.Edges {
		if edge.WayID == "100" {
			found = true
			checkScores(t, edge.RoadScores, qlearning.RoadScores{HighwayScore: 2, SurfaceScore: 1, WidthScore: 1, ComfortScore: 1.55})
		}
	}
	if !found || !strings.Contains(csv.String(), ",2,1,1,1.55") {
		t.Fatal("CSV/map used default scores")
	}
	meta := map[string]any{"status": "completed", "resolved_seeds": []uint64{18446744073709451615}, "config": map[string]any{"scenario": "B", "reward": scores}, "note": "</pre><script>bad()</script>"}
	html.Reset()
	if err := ReportToHTML(&html, "Training details", "training", meta); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html.String(), "18446744073709451615") || strings.Contains(html.String(), "</pre><script>bad()") {
		t.Fatal("report lost seed precision or text escaping")
	}
}
