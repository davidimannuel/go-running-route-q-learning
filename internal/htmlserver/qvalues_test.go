package htmlserver

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/qlearning"
)

const debugQCSV = "scenario,current_node,previous_node,progress_bin,next_node,key,q_value\nC,1,-1,0,2,0,2.5\nC,2,1,0,1,0,9\nC,2,1,0,3,0,0\nC,2,3,1,1,0,-2\nC,2,3,1,1,1,4\nC,2,3,2,,,\nC,3,2,0,2,0,1.5\n"
const debugTrace = "step,scenario,current_node,previous_node,progress_bin,action_taken,next_node,key,length_m,total_distance_m,q_before\n1,C,1,-1,0,true,2,0,100,100,2.5\n2,C,2,1,0,true,3,0,100,200,0\n3,C,3,2,0,true,2,0,100,300,1.5\n4,C,2,3,1,true,1,1,100,400,4\n"

func qFixture(t *testing.T, root string) string {
	t.Helper()
	name := "workdirs/test/output/training/profile/routes.html"
	writeFixture(t, root, name, "map")
	metadata := map[string]any{"config": qlearning.Config{Scenario: qlearning.ScenarioC, Environment: qlearning.DefaultConfig().Environment},
		"status": "completed", "resolved_seeds": []uint64{9007199254740993}, "start_node": 1, "completed_at": "2026-11-09T00:00:00Z", "access_policy_version": "edge_and_endpoint_whitelist_v1"}
	body, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.ToSlash(filepath.Dir(name))
	writeFixture(t, root, dir+"/metadata.json", string(body))
	writeFixture(t, root, dir+"/seed-9007199254740993/qtable.csv", debugQCSV)
	writeFixture(t, root, dir+"/seed-9007199254740993/evaluation-steps.csv", debugTrace)
	return name
}

func qURL(file, seed, node string) string {
	return "/q-values?" + url.Values{"file": {file}, "seed": {seed}, "node": {node}}.Encode()
}

func TestQValuesOneNodeAllContextsAndVisits(t *testing.T) {
	root := t.TempDir()
	name := qFixture(t, root)
	h := newHandler(t, root)
	w := request(h, http.MethodGet, qURL(name, "9007199254740993", "2"))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if w.Header().Get("Content-Type") != "application/json; charset=utf-8" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Header())
	}
	var d nodeQData
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.Seed != "9007199254740993" || d.Node != "2" || d.Scenario != "C" || len(d.States) != 3 || len(d.Visits) != 2 {
		t.Fatalf("response=%+v", d)
	}
	if d.States[0].PreviousNode != "1" || d.States[0].Actions[1].Value != 0 || len(d.States[2].Actions) != 0 {
		t.Fatalf("zero/empty state lost: %+v", d.States)
	}
	if d.States[1].Actions[0].Key != 0 || d.States[1].Actions[1].Key != 1 {
		t.Fatal("parallel keys lost")
	}
	if d.Visits[0].DistanceM != 100 || d.Visits[0].Step != 2 || d.Visits[0].Visited["2"] != 1 || d.Visits[0].Visited["3"] != 0 {
		t.Fatalf("first prefix=%+v", d.Visits[0])
	}
	if d.Visits[1].DistanceM != 300 || d.Visits[1].PreviousNode != "3" || d.Visits[1].Visited["2"] != 2 || d.Visits[1].Visited["3"] != 1 || d.Visits[1].Chosen.Key != 1 {
		t.Fatalf("second prefix=%+v", d.Visits[1])
	}
	if d.Visits[0].Visited["2"] != 1 {
		t.Fatal("visit maps share mutable storage")
	}
	w = request(h, http.MethodGet, qURL(name, "9007199254740993", "999"))
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil || len(d.States) != 0 || len(d.Visits) != 0 {
		t.Fatal("unseen node is not empty")
	}
	w = request(h, http.MethodHead, qURL(name, "9007199254740993", "2"))
	if w.Code != http.StatusOK || w.Body.Len() != 0 {
		t.Fatal("HEAD response")
	}
}

func TestQValuesRejectsPathsMissingArtifactsAndMixedResults(t *testing.T) {
	root := t.TempDir()
	name := qFixture(t, root)
	h := newHandler(t, root)
	for _, tc := range []struct {
		file, seed, node string
		status           int
	}{
		{name, "../1", "2", 400}, {name, "01", "2", 400}, {name, "0", "2", 404},
		{name, "9007199254740993", "-1", 400}, {name, "9007199254740993", "02", 400},
		{"workdirs/test/output/road-analysis.html", "0", "2", 400},
		{"../profile/routes.html", "0", "2", 404}, {".hidden/routes.html", "0", "2", 404},
	} {
		w := request(h, http.MethodGet, qURL(tc.file, tc.seed, tc.node))
		if w.Code != tc.status {
			t.Errorf("%+v status=%d body=%s", tc, w.Code, w.Body.String())
		}
	}
	dir := filepath.ToSlash(filepath.Dir(name)) + "/seed-9007199254740993/"
	for _, filename := range []string{"qtable.csv", "evaluation-steps.csv"} {
		if w := request(h, "GET", "/files/"+dir+filename); w.Code != 404 {
			t.Fatal("raw CSV exposed")
		}
	}
	if w := request(h, "POST", qURL(name, "9007199254740993", "2")); w.Code != 405 {
		t.Fatal("POST allowed")
	}
	writeFixture(t, root, dir+"qtable.csv", "bad\n")
	if w := request(h, "GET", qURL(name, "9007199254740993", "2")); w.Code != 422 {
		t.Fatal(w.Code)
	}
	writeFixture(t, root, dir+"qtable.csv", debugQCSV)
	writeFixture(t, root, dir+"evaluation-steps.csv", debugTrace+"5,C,1,2,1,false,,,,400,\n")
	if w := request(h, "GET", qURL(name, "9007199254740993", "1")); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	writeFixture(t, root, dir+"evaluation-steps.csv", debugTrace[:len(debugTrace)-2]+"8\n")
	if w := request(h, "GET", qURL(name, "9007199254740993", "2")); w.Code != 409 {
		t.Fatalf("mismatched Q accepted: %d %s", w.Code, w.Body.String())
	}
	writeFixture(t, root, dir+"evaluation-steps.csv", debugTrace)
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(dir+"qtable.csv"))); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "qtable.csv")
	if err := os.WriteFile(outside, []byte(debugQCSV), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, filepath.FromSlash(dir+"qtable.csv"))); err != nil {
		t.Fatal(err)
	}
	if w := request(h, "GET", qURL(name, "9007199254740993", "2")); w.Code != 404 {
		t.Fatal("symlink allowed")
	}
}
