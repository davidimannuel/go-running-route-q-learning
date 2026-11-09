package config

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTrainingDefaultsOverridesAndStrictFields(t *testing.T) {
	cfg, err := loadYAML(t, validYAML+"qlearning:\n  scenario: B\n  training:\n    episodes: 25\n    runs: 2\n")
	if err != nil {
		t.Fatal(err)
	}
	q := cfg.QLearning
	if q.Environment.MaxSteps != 220 || q.Environment.TargetDistanceM != 5000 || !q.Environment.AllowRevisit || q.Training.Alpha != 0.15 || q.Training.Episodes != 25 || q.Training.Runs == nil || *q.Training.Runs != 2 || q.Training.Seeds != nil {
		t.Fatalf("config=%+v", q)
	}
	for _, suffix := range []string{
		"qlearning:\n  scenario: D\n",
		"qlearning:\n  scenario: B\n  reward:\n    unknown: 1\n",
		"qlearning:\n  scenario: B\n  training:\n    episodes: 0\n",
		"qlearning:\n  scenario: B\n  training:\n    runs: 0\n",
		"qlearning:\n  scenario: B\n  training:\n    seeds: []\n",
		"qlearning:\n  scenario: B\n  training:\n    seeds: [0]\n    runs: 2\n",
		"qlearning:\n  scenario: B\n  training:\n    epsilon_decay_episodes: 0\n",
		"qlearning:\n  scenario: B\n  environment:\n    distance_tolerance: .nan\n",
		"qlearning:\n  scenario: B\n  experiments: []\n",
	} {
		if _, err := loadYAML(t, validYAML+suffix); err == nil {
			t.Fatalf("invalid config accepted: %s", suffix)
		}
	}
}

func TestReferenceWorkdirConfigurations(t *testing.T) {
	expected := []struct {
		name              string
		episodes, horizon int
	}{
		{"dynamic-150000", 150000, 0}, {"dynamic-300000", 300000, 0},
		{"horizon-100000-total-150000", 150000, 100000},
		{"horizon-100000-total-300000", 300000, 100000},
	}

	cfg, err := Load(filepath.Join("..", "..", "workdirs", "training"))
	if err != nil {
		t.Fatal(err)
	}
	highway := cfg.QLearning.Reward.Road.Highway
	if highway.SecondaryPriority == nil || *highway.SecondaryPriority != 0.5 {
		t.Fatalf("workdir highway preferences mismatch: %+v", highway)
	}
	training := cfg.QLearning.Training
	if cfg.QLearning.Scenario != "" || fmt.Sprint(training.Seeds) != "[0 1 2 3 4]" || training.MaxWorkers != 5 || len(training.Profiles) != 3*len(expected) {
		t.Fatalf("shared settings mismatch: %+v", training)
	}
	for si, scenario := range []string{"a", "b", "c"} {
		for i, want := range expected {
			got := training.Profiles[si*len(expected)+i]
			if got.Name != scenario+"-"+want.name || string(got.Scenario) != strings.ToUpper(scenario) || got.Episodes != want.episodes {
				t.Fatalf("profile mismatch: %+v", got)
			}
			if want.horizon == 0 && got.EpsilonDecayEpisodes != nil || want.horizon != 0 && (got.EpsilonDecayEpisodes == nil || *got.EpsilonDecayEpisodes != want.horizon) {
				t.Fatalf("horizon mismatch: %+v", got)
			}
		}
	}
	testing, err := Load(filepath.Join("..", "..", "workdirs", "testing"))
	if err != nil {
		t.Fatal(err)
	}
	testTraining := testing.QLearning.Training
	if testTraining.Seeds != nil || testTraining.Runs == nil || *testTraining.Runs != 5 || testTraining.MaxWorkers != 5 {
		t.Fatal("testing must share five automatic seeds across all profiles")
	}
	// The testing workdir may select fewer of the same experiment profiles.
	for _, selected := range testTraining.Profiles {
		found := false
		for _, reference := range training.Profiles {
			if reflect.DeepEqual(selected, reference) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("testing profile differs from training definition: %+v", selected)
		}
	}
	if testing.QLearning.Environment != cfg.QLearning.Environment || !testing.QLearning.Reward.Equal(cfg.QLearning.Reward) {
		t.Fatal("testing environment and reward must match training")
	}

}

func TestTrainingProfileValidation(t *testing.T) {
	for _, profileYAML := range []string{
		"[]",
		"[{name: duplicate, episodes: 5}, {name: duplicate, episodes: 10}]",
		"[{name: ../escape, episodes: 5}]",
		"[{name: /absolute, episodes: 5}]",
		"[{name: '-leading', episodes: 5}]",
		"[{name: Invalid, episodes: 5}]",
		"[{episodes: 5}]",
		"[{name: short, episodes: 0}]",
		"[{name: short, episodes: 5, epsilon_decay_episodes: 0}]",
		"[{name: short, episodes: 5, typo: true}]",
	} {
		content := validYAML + "qlearning:\n  scenario: B\n  training:\n    profiles: " + profileYAML + "\n"
		if _, err := loadYAML(t, content); err == nil {
			t.Fatalf("invalid profiles accepted: %s", profileYAML)
		}
	}
}
