package config

import "testing"

func TestExportModeDefaultsAndValidation(t *testing.T) {
	for _, mode := range []string{"", "standard", "advanced"} {
		yaml := validYAML
		if mode != "" {
			yaml += "export:\n  map_mode: " + mode + "\n"
		}
		cfg, err := loadYAML(t, yaml)
		if err != nil {
			t.Fatal(err)
		}
		want := mode
		if want == "" {
			want = "advanced"
		}
		if cfg.Export.Mode() != want {
			t.Fatalf("mode=%q, want %q", cfg.Export.Mode(), want)
		}
	}
	for _, suffix := range []string{"export:\n  map_mode: simple\n", "export:\n  map_mode: ''\n  typo: true\n"} {
		if _, err := loadYAML(t, validYAML+suffix); err == nil {
			t.Fatal("invalid export config accepted")
		}
	}
}
