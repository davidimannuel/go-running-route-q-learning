// Package htmlserver serves local HTML artifacts through an explicit file allowlist.
package htmlserver

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
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
	http.ServeContent(w, r, path.Base(name), info.ModTime(), file)
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
	Name string
	URL  string
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
			entries = append(entries, indexEntry{Name: name, URL: link.String()})
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
