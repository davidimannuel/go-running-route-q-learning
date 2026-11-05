package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
	"go.yaml.in/yaml/v3"
)

const Filename = "config.yaml"

// Config contains the settings shared by commands operating on a workdir.
type Config struct {
	SchemaVersion int
	Map           MapConfig
	OutputDir     string
}

type MapConfig struct {
	Latitude   float64
	Longitude  float64
	RadiusM    float64
	TimeoutSec int
}

func (m MapConfig) FetchParam() osm.FetchParam {
	return osm.FetchParam{Lat: m.Latitude, Long: m.Longitude, RadM: m.RadiusM, TimeoutSec: m.TimeoutSec}
}

// Pointer fields distinguish omitted coordinates from valid zero values.
type configFile struct {
	SchemaVersion *int     `yaml:"schema_version"`
	Map           *mapFile `yaml:"map"`
	OutputDir     *string  `yaml:"output_dir"`
}

type mapFile struct {
	Latitude   *float64 `yaml:"latitude"`
	Longitude  *float64 `yaml:"longitude"`
	RadiusM    *float64 `yaml:"radius_m"`
	TimeoutSec *int     `yaml:"timeout_sec"`
}

// Load reads and validates the single config.yaml document in workdir.
func Load(workdir string) (Config, error) {
	if strings.TrimSpace(workdir) == "" {
		return Config{}, fmt.Errorf("config: workdir is required")
	}
	path := filepath.Join(workdir, Filename)
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("config: open %s: %w", path, err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var raw configFile
	if err := decoder.Decode(&raw); err != nil {
		return Config{}, fmt.Errorf("config: decode %s: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return Config{}, fmt.Errorf("config: decode trailing document: %w", err)
		}
		return Config{}, fmt.Errorf("config: exactly one YAML document is required")
	}

	if raw.SchemaVersion == nil || *raw.SchemaVersion != 1 {
		return Config{}, fmt.Errorf("config: schema_version is required and must be 1")
	}
	if raw.Map == nil {
		return Config{}, fmt.Errorf("config: map is required")
	}
	for _, field := range []struct {
		name    string
		missing bool
	}{
		{"latitude", raw.Map.Latitude == nil},
		{"longitude", raw.Map.Longitude == nil},
		{"radius_m", raw.Map.RadiusM == nil},
		{"timeout_sec", raw.Map.TimeoutSec == nil},
	} {
		if field.missing {
			return Config{}, fmt.Errorf("config: map.%s is required", field.name)
		}
	}

	cfg := Config{
		SchemaVersion: *raw.SchemaVersion,
		Map: MapConfig{
			Latitude:   *raw.Map.Latitude,
			Longitude:  *raw.Map.Longitude,
			RadiusM:    *raw.Map.RadiusM,
			TimeoutSec: *raw.Map.TimeoutSec,
		},
		OutputDir: "output",
	}
	if raw.OutputDir != nil {
		cfg.OutputDir = strings.TrimSpace(*raw.OutputDir)
	}
	if !filepath.IsLocal(cfg.OutputDir) {
		return Config{}, fmt.Errorf("config: output_dir must be a non-empty relative path within workdir")
	}
	cfg.OutputDir = filepath.Clean(cfg.OutputDir)
	if err := cfg.Map.FetchParam().Validate(); err != nil {
		return Config{}, fmt.Errorf("config: invalid map settings: %w", err)
	}
	return cfg, nil
}
