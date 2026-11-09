// Package htmlserver serves local HTML artifacts through an explicit file allowlist.
package htmlserver

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/davidimannuel/go-running-route-q-learning/internal/analysis"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/davidimannuel/go-running-route-q-learning/internal/logger"
)

//go:embed index.gohtml
var indexHTML string

var indexTemplate = template.Must(template.New("index").Parse(indexHTML))

// Handler owns a rooted filesystem. Call Close only after HTTP serving stops.
type Handler struct {
	root *os.Root
}

// New opens the selected directory without reading any experiment configuration.
// os.Root enforces the directory boundary even if filesystem entries are replaced.
func New(directory string) (*Handler, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, fmt.Errorf("open HTML directory: %w", err)
	}
	return &Handler{root: root}, nil
}

func (h *Handler) Close() error {
	return h.root.Close()
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// A normal HTTP referrer is needed by providers such as OSM's tile service.
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	logger.Debug(r.Context(), "HTML request", logger.String("method", r.Method), logger.String("path", r.URL.Path))
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path == "/q-values" {
		h.serveQValues(w, r)
		return
	}
	if r.URL.Path == "/details" {
		h.serveDetails(w, r)
		return
	}
	if r.URL.Path == "/" || r.URL.Path == "/index.html" {
		h.serveIndex(w, r)
		return
	}
	// Files have their own namespace so an artifact named index.html remains accessible.
	name, ok := strings.CutPrefix(r.URL.Path, "/files/")
	if !ok || !allowedPath(name) || !h.isRegularFile(name) {
		http.NotFound(w, r)
		return
	}
	file, err := h.root.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Local artifacts are overwritten in place; always send the current HTML,
	// even when a regenerated file keeps its previous modification timestamp.
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, path.Base(name), time.Time{}, file)
}

// allowedPath rejects traversal and hidden entries before touching the filesystem.
// Only .html files are served; the same rule is used for listing and direct URLs.
func allowedPath(name string) bool {
	if !fs.ValidPath(name) || strings.Contains(name, "\\") || !strings.EqualFold(path.Ext(name), ".html") {
		return false
	}
	for part := range strings.SplitSeq(name, "/") {
		if strings.HasPrefix(part, ".") {
			return false
		}
	}
	return true
}

// Reject symlinks in every path component, including links to non-HTML files
// inside the root. Rooted opening provides a separate boundary against escapes.
func (h *Handler) isRegularFile(name string) bool {
	parts := strings.Split(name, "/")
	for i := range parts {
		info, err := h.root.Lstat(strings.Join(parts[:i+1], "/"))
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
		if i < len(parts)-1 && !info.IsDir() {
			return false
		}
		if i == len(parts)-1 {
			return info.Mode().IsRegular()
		}
	}
	return false
}

type indexEntry struct {
	Name, URL, MapURL, DetailsURL, Group, Title, Kind, Summary string
}

func (h *Handler) serveIndex(w http.ResponseWriter, r *http.Request) {
	entries := make([]indexEntry, 0)
	// Walk on each index request so new analyze outputs appear without a restart.
	// WalkDir sorts entries and does not follow symbolic links.
	err := fs.WalkDir(h.root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if err := r.Context().Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if name == "." {
			return nil
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		if entry.Type().IsRegular() && allowedPath(name) && h.isRegularFile(name) {
			// URL encoding and template escaping handle spaces, #, ?, and HTML in names.
			link := url.URL{Path: "/files/" + name}
			item := indexEntry{Name: name, URL: link.String(), Title: entry.Name(), Group: path.Dir(name), Kind: "other"}
			if entry.Name() == "routes.html" || entry.Name() == "road-analysis.html" {
				item.Kind = "map"
				item.MapURL = item.URL
				if entry.Name() == "routes.html" {
					item.Title = path.Base(path.Dir(name)) + " · Routes"
				} else {
					item.Title = "Road analysis"
				}
				if document, _, err := h.readMetadata(name); err == nil {
					detail := url.URL{Path: "/details", RawQuery: url.Values{"file": {name}}.Encode()}
					item.DetailsURL = detail.String()
					item.Summary = metadataSummary(document)
				}
			} else if entry.Name() == "report.html" || entry.Name() == "analysis.html" {
				item.Kind = "details"
				item.Title = path.Base(path.Dir(name)) + " · Details"
				item.DetailsURL = item.URL
				mapName := "routes.html"
				if entry.Name() == "analysis.html" {
					mapName = "road-analysis.html"
				}
				candidate := path.Join(path.Dir(name), mapName)
				if h.isRegularFile(candidate) {
					link := url.URL{Path: "/files/" + candidate}
					item.MapURL = link.String()
				}
			}
			entries = append(entries, item)
		}
		return nil
	})
	if err != nil {
		logger.Error(r.Context(), "Could not list HTML files", logger.Err(err))
		http.Error(w, "Could not list HTML files", http.StatusInternalServerError)
		return
	}
	var body bytes.Buffer
	if err := indexTemplate.Execute(&body, entries); err != nil {
		logger.Error(r.Context(), "Could not render HTML index", logger.Err(err))
		http.Error(w, "Could not render HTML index", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(body.Bytes()))
}

// Metadata is rendered into HTML, never exposed as an arbitrary file download.
// The requested map and its adjacent metadata must both be regular rooted files.
func (h *Handler) readMetadata(name string) (map[string]any, string, error) {
	if !allowedPath(name) || !h.isRegularFile(name) {
		return nil, "", fmt.Errorf("invalid map path")
	}
	kind, filename := "training", "metadata.json"
	switch path.Base(name) {
	case "routes.html":
	case "road-analysis.html":
		kind, filename = "analysis", "analysis.json"
	default:
		return nil, "", fmt.Errorf("no metadata for this HTML file")
	}
	metadataPath := path.Join(path.Dir(name), filename)
	if !h.isRegularFile(metadataPath) {
		return nil, "", fmt.Errorf("metadata unavailable")
	}
	file, err := h.root.Open(metadataPath)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() > 2<<20 {
		return nil, "", fmt.Errorf("invalid metadata file")
	}
	decoder := json.NewDecoder(io.LimitReader(file, 2<<20))
	decoder.UseNumber()
	var document map[string]any
	if err := decoder.Decode(&document); err != nil {
		return nil, "", err
	}
	// A failed rerun can coexist with a previous completed result. Keep both
	// identities visible instead of presenting that old result as the new attempt.
	directory := path.Dir(name)
	marker := path.Join(path.Dir(directory), path.Base(directory)+"-run.json")
	if h.isRegularFile(marker) {
		if file, err := h.root.Open(marker); err == nil {
			var latest map[string]any
			decoder := json.NewDecoder(io.LimitReader(file, 2<<20))
			decoder.UseNumber()
			if err := decoder.Decode(&latest); err == nil && (latest["status"] != "completed" || latest["completed_at"] != document["completed_at"]) {
				document["latest_attempt"] = latest
			}
			file.Close()
		}
	}
	return document, kind, nil
}

func metadataSummary(document map[string]any) string {
	scenario, episodes := "", ""
	if cfg, ok := document["config"].(map[string]any); ok {
		scenario, _ = cfg["scenario"].(string)
		if training, ok := cfg["training"].(map[string]any); ok && training["episodes"] != nil {
			episodes = fmt.Sprint(training["episodes"])
		}
	}
	status, _ := document["status"].(string)
	if scenario != "" {
		summary := fmt.Sprintf("Scenario %s · %s episodes · %s", scenario, episodes, status)
		if latest, ok := document["latest_attempt"].(map[string]any); ok {
			summary += fmt.Sprintf(" · latest attempt: %v (previous result shown)", latest["status"])
		}
		return summary
	}
	return status
}

func (h *Handler) serveDetails(w http.ResponseWriter, r *http.Request) {
	document, kind, err := h.readMetadata(r.URL.Query().Get("file"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var body bytes.Buffer
	if err := analysis.ReportToHTML(&body, "Recorded result details", kind, document); err != nil {
		http.Error(w, "Could not render metadata", http.StatusInternalServerError)
		return
	}
	// Resolve relative map links against the map's actual directory.
	base := url.URL{Path: "/files/" + path.Dir(r.URL.Query().Get("file")) + "/"}
	rendered := strings.Replace(body.String(), "<head>", `<head><base href="`+template.HTMLEscapeString(base.String())+`">`, 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "details.html", time.Time{}, strings.NewReader(rendered))
}
