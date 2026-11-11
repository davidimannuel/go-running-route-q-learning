package analysis

import (
	_ "embed"
	"html/template"
	"io"
)

// SiteSeed keeps uint64 identifiers as text throughout the browser workflow.
type SiteSeed struct {
	ID, Distance, Result, URL string
	IsLoop, Representative    bool
}

type SiteEntry struct {
	Name, Kind, Scenario, MapURL, DetailsURL, CompletedAt string
	Seeds                                                 []SiteSeed
}

//go:embed site.gohtml
var siteHTML string

var siteTemplate = template.Must(template.New("site").Parse(siteHTML))

func SiteToHTML(w io.Writer, title, mode string, entries []SiteEntry) error {
	return siteTemplate.Execute(w, struct {
		Title, Mode string
		Entries     []SiteEntry
	}{title, mode, entries})
}
