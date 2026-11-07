package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
	"github.com/davidimannuel/go-running-route-q-learning/internal/qlearning"
	"go.yaml.in/yaml/v3"
)

const Filename = "config.yaml"

// Config contains the settings shared by commands operating on a workdir.
type Config struct {
	SchemaVersion int              `json:"schema_version"`
	Map           MapConfig        `json:"map"`
	OutputDir     string           `json:"output_dir"`
	CacheDir      string           `json:"cache_dir"`
	QLearning     qlearning.Config `json:"qlearning"`
}

type MapConfig struct {
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	RadiusM    float64 `json:"radius_m"`
	TimeoutSec int     `json:"timeout_sec"`
}

func (m MapConfig) FetchParam() osm.FetchParam {
	return osm.FetchParam{Lat: m.Latitude, Long: m.Longitude, RadM: m.RadiusM, TimeoutSec: m.TimeoutSec}
}

// Pointer fields distinguish omitted coordinates from valid zero values.
type configFile struct {
	SchemaVersion *int              `yaml:"schema_version"`
	Map           *mapFile          `yaml:"map"`
	OutputDir     *string           `yaml:"output_dir"`
	CacheDir      *string           `yaml:"cache_dir"`
	QLearning     *qlearning.Config `yaml:"qlearning"`
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
	defaults := qlearning.DefaultConfig()
	raw := configFile{QLearning: &defaults}
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
		CacheDir:  "cache",
	}
	cfg.QLearning = defaults
	if raw.QLearning != nil {
		cfg.QLearning = *raw.QLearning
	}
	// Map-only workdirs remain valid. Train explicitly requires scenario A/B/C.
	if err := cfg.QLearning.Reward.Validate(); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	if cfg.QLearning.Scenario != "" || cfg.QLearning.Training.Profiles != nil {
		if err := cfg.QLearning.Validate(); err != nil {
			return Config{}, fmt.Errorf("config: %w", err)
		}
	}
	if raw.OutputDir != nil {
		cfg.OutputDir = strings.TrimSpace(*raw.OutputDir)
	}
	if raw.CacheDir != nil {
		cfg.CacheDir = strings.TrimSpace(*raw.CacheDir)
	}
	for _, directory := range []struct{ name, value string }{
		{"output_dir", cfg.OutputDir},
		{"cache_dir", cfg.CacheDir},
	} {
		if !filepath.IsLocal(directory.value) {
			return Config{}, fmt.Errorf("config: %s must be a non-empty relative path within workdir", directory.name)
		}
	}
	cfg.OutputDir = filepath.Clean(cfg.OutputDir)
	cfg.CacheDir = filepath.Clean(cfg.CacheDir)
	if err := cfg.Map.FetchParam().Validate(); err != nil {
		return Config{}, fmt.Errorf("config: invalid map settings: %w", err)
	}
	return cfg, nil
}
