package htmlserver

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordedMetadataNavigationAndPrecision(t *testing.T) {
	root := t.TempDir()
	name := "training/output/training/b/routes.html"
	writeFixture(t, root, name, "<title>Route</title>")
	writeFixture(t, root, "training/output/training/b/metadata.json", `{"status":"completed","resolved_seeds":[18446744073709451615],"config":{"scenario":"B","reward":{"revisit_penalty":-3},"training":{"episodes":4}},"snapshot_sha256":"abc","note":"</pre><script>alert(1)</script>"}`)
	h := newHandler(t, root)
	index := request(h, "GET", "/").Body.String()
	if !strings.Contains(index, "Scenario B") || !strings.Contains(index, "/details?file=") || !strings.Contains(index, "b · Routes") {
		t.Fatal("missing profile navigation", index)
	}
	target := "/details?" + url.Values{"file": {name}}.Encode()
	w := request(h, "GET", target)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "18446744073709451615") || !strings.Contains(w.Body.String(), "revisit_penalty") || !strings.Contains(w.Body.String(), `<base href="/files/training/output/training/b/">`) {
		t.Fatal("missing recorded metadata", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "</pre><script>alert(1)</script>") {
		t.Fatal("metadata HTML injection")
	}
	for _, bad := range []string{"../outside.html", "training/output/training/b/metadata.json", ".git/private.html", "missing.html"} {
		w := request(h, "GET", "/details?"+url.Values{"file": {bad}}.Encode())
		if w.Code != 404 {
			t.Fatalf("accepted metadata path %s", bad)
		}
	}
	if w := request(h, "GET", "/files/training/output/training/b/metadata.json"); w.Code != 404 {
		t.Fatal("JSON exposed as a static file")
	}
	outside := filepath.Join(t.TempDir(), "metadata.json")
	if err := os.WriteFile(outside, []byte(`{"status":"secret"}`), 0600); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(root, "training/output/training/b/metadata.json")
	if err := os.Remove(local); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, local); err != nil {
		t.Fatal(err)
	}
	if w := request(h, "GET", target); w.Code != 404 {
		t.Fatal("followed metadata symlink")
	}
}

func TestFailedAttemptKeepsPreviousResultIdentifiable(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "output/training/b/routes.html", "map")
	writeFixture(t, root, "output/training/b/metadata.json", `{"status":"completed","completed_at":"old","config":{"scenario":"B","training":{"episodes":4}}}`)
	writeFixture(t, root, "output/training/b-run.json", `{"status":"failed","error":"interrupted","config":{"scenario":"B","training":{"episodes":8}}}`)
	h := newHandler(t, root)
	body := request(h, "GET", "/").Body.String()
	if !strings.Contains(body, "latest attempt: failed") {
		t.Fatal("failed rerun hidden")
	}
	detail := request(h, "GET", "/details?file=output/training/b/routes.html").Body.String()
	if !strings.Contains(detail, "interrupted") || !strings.Contains(detail, "previous") && !strings.Contains(detail, "Latest attempt") {
		t.Fatal("missing failed attempt context")
	}
}
