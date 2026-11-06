package osm

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
)

// Load reads a successful Overpass JSON snapshot from disk. Unmarshal rejects
// trailing JSON documents, while validation rejects incomplete/error responses.
// It performs no HTTP request and does not modify the snapshot.
func Load(path string) (OSMData, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return OSMData{}, fmt.Errorf("load OSM: read %s: %w", path, err)
	}
	data, err := Parse(body)
	if err != nil {
		return OSMData{}, fmt.Errorf("load OSM: %s: %w", path, err)
	}
	return data, nil
}

// Parse validates the exact Overpass response bytes used for a reproducible
// snapshot hash. It shares validation with disk loading and cache handling.
func Parse(body []byte) (OSMData, error) {
	var data OSMData
	if err := json.Unmarshal(body, &data); err != nil {
		return OSMData{}, fmt.Errorf("decode OSM: %w", err)
	}
	if err := validateOSMData(data); err != nil {
		return OSMData{}, fmt.Errorf("invalid OSM snapshot: %w", err)
	}
	return data, nil
}

// validateOSMData is shared by local snapshots and cached Overpass responses.
func validateOSMData(data OSMData) error {
	if data.Version <= 0 || math.IsNaN(data.Version) || math.IsInf(data.Version, 0) {
		return fmt.Errorf("missing or invalid OSM version")
	}
	if data.Elements == nil {
		return fmt.Errorf("elements must be a JSON array")
	}
	if strings.TrimSpace(data.Remark) != "" {
		return fmt.Errorf("server remark: %s", data.Remark)
	}
	return nil
}
