package htmlserver

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFixture(t *testing.T, root, name, content string) {
	t.Helper()
	filename := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func newHandler(t *testing.T, root string) *Handler {
	t.Helper()
	h, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := h.Close(); err != nil {
			t.Error(err)
		}
	})
	return h
}

func request(h http.Handler, method, target string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, target, nil))
	return w
}

func TestIndexListsOnlyHTMLAndRefreshes(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"index.html":                        "artifact index",
		"playground/map.html":               "map",
		"workdirs/a/output/route.HTML":      "route",
		"workdirs/a/output/edges.csv":       "private CSV",
		"workdirs/a/config.yaml":            "private config",
		".git/private.html":                 "hidden",
		".venv/private.html":                "hidden",
		"playground/.hidden.html":           "hidden",
		"playground/map.html.backup":        "backup",
		"playground/space & #?.html":        "special name",
		"<img src=x onerror=alert(1)>.html": "escaped name",
	} {
		writeFixture(t, root, name, content)
	}
	h := newHandler(t, root)
	for _, target := range []string{"/", "/index.html"} {
		w := request(h, http.MethodGet, target)
		body := w.Body.String()
		if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("index status=%d, headers=%v", w.Code, w.Header())
		}
		for _, want := range []string{"playground/map.html", "workdirs/a/output/route.HTML", "/files/index.html", "space &amp; #?.html", "&lt;img"} {
			if !strings.Contains(body, want) {
				t.Errorf("index does not contain %q: %s", want, body)
			}
		}
		for _, hidden := range []string{"edges.csv", "config.yaml", ".git/", ".venv/", ".hidden.html", "map.html.backup", "<img src=x"} {
			if strings.Contains(body, hidden) {
				t.Errorf("index exposes %q", hidden)
			}
		}
		if strings.Index(body, "/files/index.html") > strings.Index(body, "/files/playground/map.html") {
			t.Error("index entries are not sorted by relative path")
		}
	}
	writeFixture(t, root, "new.html", "new output")
	if body := request(h, http.MethodGet, "/").Body.String(); !strings.Contains(body, "/files/new.html") {
		t.Fatal("new HTML output requires a server restart")
	}
}

func TestHTMLContentsAndEncodedNames(t *testing.T) {
	root := t.TempDir()
	h := newHandler(t, root)
	for _, name := range []string{"map.html", "index.html", "nested/space & #?%.html", "nested/map.HTML"} {
		writeFixture(t, root, name, "<!doctype html><title>Graph</title>")
		link := url.URL{Path: "/files/" + name}
		w := request(h, http.MethodGet, link.String())
		if w.Code != http.StatusOK || w.Body.String() != "<!doctype html><title>Graph</title>" {
			t.Fatalf("name=%q, status=%d, body=%q", name, w.Code, w.Body.String())
		}
		if w.Header().Get("Content-Type") != "text/html; charset=utf-8" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("unexpected content headers: %v", w.Header())
		}
		if w.Header().Get("Referrer-Policy") != "strict-origin-when-cross-origin" {
			t.Fatal("HTML responses should allow a normal HTTP origin referrer")
		}
		w = request(h, http.MethodHead, link.String())
		if w.Code != http.StatusOK || w.Body.Len() != 0 {
			t.Fatalf("HEAD status=%d, body=%q", w.Code, w.Body.String())
		}
	}
	if w := request(h, http.MethodHead, "/index.html"); w.Code != http.StatusOK || w.Body.Len() != 0 {
		t.Fatalf("index HEAD status=%d, body=%q", w.Code, w.Body.String())
	}
}

func TestRejectsFilesOutsideAllowlist(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	writeFixture(t, root, "nested/map.html", "map")
	writeFixture(t, root, "data.csv", "private CSV")
	writeFixture(t, root, "config.yaml", "private config")
	writeFixture(t, root, "osm.json", "private snapshot")
	writeFixture(t, root, ".git/private.html", "hidden HTML")
	writeFixture(t, parent, "outside.html", "outside HTML")
	h := newHandler(t, root)
	for _, target := range []string{
		"/files/data.csv", "/files/config.yaml", "/files/osm.json",
		"/files/data.csv?name=map.html", "/files/.git/private.html",
		"/files/../outside.html", "/files/%2e%2e/outside.html",
		"/files/nested/../../outside.html", "/files/nested%5c..%5c..%5coutside.html",
		"/files//nested/map.html", "/files/nested/./map.html",
		"/files/missing.html", "/files/nested", "/files/nested/", "/files/",
		"/nested/map.html", "/config.yaml",
	} {
		t.Run(target, func(t *testing.T) {
			w := request(h, http.MethodGet, target)
			if w.Code != http.StatusNotFound {
				t.Fatalf("status=%d; want 404: %s", w.Code, w.Body.String())
			}
		})
	}
	if w := request(h, http.MethodPost, "/files/nested/map.html"); w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST status=%d, headers=%v", w.Code, w.Header())
	}
}

func TestRejectsSymlinkFilesAndDirectories(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeFixture(t, root, "data.csv", "private CSV")
	writeFixture(t, root, "nested/map.html", "map")
	writeFixture(t, outside, "private.html", "outside HTML")
	for link, target := range map[string]string{
		"csv-link.html":  "data.csv",
		"html-link.html": "nested/map.html",
		"directory-link": "nested",
		"outside-link":   outside,
		"outside.html":   filepath.Join(outside, "private.html"),
	} {
		if err := os.Symlink(target, filepath.Join(root, link)); err != nil {
			t.Fatal(err)
		}
	}
	h := newHandler(t, root)
	for _, target := range []string{"/files/csv-link.html", "/files/html-link.html", "/files/directory-link/map.html", "/files/outside-link/private.html", "/files/outside.html"} {
		if w := request(h, http.MethodGet, target); w.Code != http.StatusNotFound {
			t.Fatalf("symlink %q returned %d", target, w.Code)
		}
	}
	body := request(h, http.MethodGet, "/").Body.String()
	for _, name := range []string{"csv-link.html", "html-link.html", "directory-link", "outside-link", "outside.html"} {
		if strings.Contains(body, name) {
			t.Errorf("index exposes symlink %q", name)
		}
	}
}

func TestEmptyAndInvalidDirectory(t *testing.T) {
	root := t.TempDir()
	h := newHandler(t, root)
	if body := request(h, http.MethodGet, "/").Body.String(); !strings.Contains(body, "No HTML files found") {
		t.Fatalf("missing empty state: %s", body)
	}
	writeFixture(t, root, "map.html", "map")
	for _, name := range []string{filepath.Join(root, "missing"), filepath.Join(root, "map.html")} {
		if h, err := New(name); err == nil {
			h.Close()
			t.Fatalf("New(%q) should fail", name)
		}
	}
}

func TestLocalHTTPNavigation(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "playground/map.html", "<!doctype html><title>Local map</title>")
	server := httptest.NewServer(newHandler(t, root))
	defer server.Close()
	resp, err := server.Client().Get(server.URL + "/files/playground/map.html")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	content, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(content), "Local map") {
		t.Fatalf("status=%d, body=%q, error=%v", resp.StatusCode, content, err)
	}
}

func TestHTMLRefreshIgnoresOldModificationTime(t *testing.T) {
	root := t.TempDir()
	h := newHandler(t, root)
	writeFixture(t, root, "map.html", "old map")
	modified := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	file := filepath.Join(root, "map.html")
	if err := os.Chtimes(file, modified, modified); err != nil {
		t.Fatal(err)
	}
	if got := request(h, http.MethodGet, "/files/map.html"); got.Code != http.StatusOK || got.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("HTML must not be cached: %d, %v", got.Code, got.Header())
	}
	writeFixture(t, root, "map.html", "new map with node access")
	if err := os.Chtimes(file, modified, modified); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req := httptest.NewRequest(method, "/files/map.html", nil)
		req.Header.Set("If-Modified-Since", modified.Format(http.TimeFormat))
		response := httptest.NewRecorder()
		h.ServeHTTP(response, req)
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s returned stale conditional response: %d, %v", method, response.Code, response.Header())
		}
		if method == http.MethodGet && response.Body.String() != "new map with node access" {
			t.Fatal("regenerated HTML was not served")
		}
		if method == http.MethodHead && response.Body.Len() != 0 {
			t.Fatal("HEAD must not send HTML body")
		}
	}
}
