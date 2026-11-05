package osm

import (
	"strconv"
	"time"
)

type OSMData struct {
	Version   float64   `json:"version"`
	Generator string    `json:"generator"`
	Remark    string    `json:"remark,omitempty"`
	Osm3S     Osm3S     `json:"osm3s"`
	Elements  []Element `json:"elements"`
}

type Osm3S struct {
	TimestampOsmBase time.Time `json:"timestamp_osm_base"`
	Copyright        string    `json:"copyright"`
}

const (
	TypeNode = "node"
	TypeWay  = "way"
)

type Element struct {
	ID    int     `json:"id"`
	Type  string  `json:"type"` // node, way
	Tags  Tags    `json:"tags,omitempty"`
	Lat   float64 `json:"lat,omitempty"`
	Lon   float64 `json:"lon,omitempty"`
	Nodes []int   `json:"nodes,omitempty"`
}

// Tags stores the original OSM key/value attributes without normalization.
type Tags map[string]string

// Get returns an empty string for missing tags; reads from a nil map are valid in Go.
func (t Tags) Get(k string) string {
	return t[k]
}

// GetFloat64 parses a plain numeric tag, returning zero for missing or invalid values.
// Units and combined values such as "3 m" or "3;4" are not parsed by this helper.
func (t Tags) GetFloat64(k string) float64 {
	val, err := strconv.ParseFloat(t.Get(k), 64)
	if err == nil {
		return val
	}
	return 0
}
