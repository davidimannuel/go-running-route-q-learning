package osm

import "time"

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

type Element struct {
	Type  string            `json:"type"`
	ID    int               `json:"id"`
	Lat   float64           `json:"lat,omitempty"`
	Lon   float64           `json:"lon,omitempty"`
	Tags  map[string]string `json:"tags,omitempty"`
	Nodes []int             `json:"nodes,omitempty"`
}
