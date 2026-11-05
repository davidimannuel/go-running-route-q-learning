package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
)

const validYAML = `schema_version: 1
map:
  latitude: -7.282187
  longitude: 112.709282
  radius_m: 3025
  timeout_sec: 90
`

func loadYAML(t *testing.T, content string) (Config, error) {
	t.Helper()
	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, Filename), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return Load(workdir)
}

func TestLoad(t *testing.T) {
	for _, tc := range []struct{ name, suffix, want string }{
		{"default output", "", "output"},
		{"custom output", "output_dir: results\n", "results"},
		{"nested output", "output_dir: results/baseline\n", filepath.Join("results", "baseline")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := loadYAML(t, validYAML+tc.suffix)
			if err != nil {
				t.Fatal(err)
			}
			want := osm.FetchParam{Lat: -7.282187, Long: 112.709282, RadM: 3025, TimeoutSec: 90}
			if cfg.SchemaVersion != 1 || cfg.OutputDir != tc.want || cfg.Map.FetchParam() != want {
				t.Fatalf("unexpected config: %+v", cfg)
			}
		})
	}
}

func TestLoadAllowsZeroCoordinates(t *testing.T) {
	content := strings.ReplaceAll(validYAML, "-7.282187", "0")
	content = strings.ReplaceAll(content, "112.709282", "0")
	cfg, err := loadYAML(t, content)
	if err != nil || cfg.Map.Latitude != 0 || cfg.Map.Longitude != 0 {
		t.Fatalf("config=%+v, error=%v", cfg, err)
	}
}

func TestLoadRejectsInvalidConfig(t *testing.T) {
	cases := []struct{ name, content string }{
		{"empty file", ""},
		{"missing version", strings.ReplaceAll(validYAML, "schema_version: 1\n", "")},
		{"unsupported version", strings.ReplaceAll(validYAML, "schema_version: 1", "schema_version: 2")},
		{"missing map", "schema_version: 1\n"},
		{"missing latitude", strings.ReplaceAll(validYAML, "  latitude: -7.282187\n", "")},
		{"null latitude", strings.ReplaceAll(validYAML, "latitude: -7.282187", "latitude: null")},
		{"missing longitude", strings.ReplaceAll(validYAML, "  longitude: 112.709282\n", "")},
		{"missing radius", strings.ReplaceAll(validYAML, "  radius_m: 3025\n", "")},
		{"missing timeout", strings.ReplaceAll(validYAML, "  timeout_sec: 90\n", "")},
		{"unknown map field", validYAML + "  radius: 300\n"},
		{"unknown root field", validYAML + "ouput_dir: results\n"},
		{"duplicate field", validYAML + "schema_version: 1\n"},
		{"multiple documents", validYAML + "---\n" + validYAML},
		{"invalid YAML", "schema_version: [\n"},
		{"invalid latitude", strings.ReplaceAll(validYAML, "-7.282187", "112.709282")},
		{"invalid longitude", strings.ReplaceAll(validYAML, "112.709282", "181")},
		{"NaN latitude", strings.ReplaceAll(validYAML, "-7.282187", ".nan")},
		{"zero radius", strings.ReplaceAll(validYAML, "radius_m: 3025", "radius_m: 0")},
		{"negative timeout", strings.ReplaceAll(validYAML, "timeout_sec: 90", "timeout_sec: -1")},
		{"blank output", validYAML + "output_dir: ''\n"},
		{"absolute output", validYAML + "output_dir: /tmp/results\n"},
		{"escaping output", validYAML + "output_dir: ../results\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := loadYAML(t, tc.content); err == nil {
				t.Fatal("expected a configuration error")
			}
		})
	}
}

func TestLoadMissingWorkdir(t *testing.T) {
	for _, workdir := range []string{"", t.TempDir()} {
		if _, err := Load(workdir); err == nil {
			t.Fatalf("expected an error for workdir %q", workdir)
		}
	}
}
