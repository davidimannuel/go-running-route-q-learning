package config

import (
	"strings"
	"testing"
)

func TestRewardOverridesPreserveDefaultsAndAllowZero(t *testing.T) {
	cfg, err := loadYAML(t, validYAML+`qlearning:
  reward:
    no_action_penalty: 0
    revisit_penalty: -3
    road:
      highway:
        priority: 2
  training:
    profiles:
      - {name: a, scenario: A, episodes: 3}
      - {name: b, scenario: B, episodes: 3}
      - {name: c, scenario: C, episodes: 3}
`)
	if err != nil {
		t.Fatal(err)
	}
	q := cfg.QLearning
	if q.Reward.NoActionPenalty != 0 || q.Reward.RevisitPenalty != -3 || q.Reward.SuccessBonus != 100 || q.Reward.Road.Highway.Priority != 2 || q.Reward.Road.Highway.Weight != 0.55 || q.Reward.Road.Surface.Missing != -0.5 {
		t.Fatalf("override/default mismatch: %+v", q.Reward)
	}
	for _, profile := range q.Training.Profiles {
		if q.ForProfile(profile).Scenario != profile.Scenario {
			t.Fatal("scenario not resolved")
		}
	}
	for _, field := range []string{"return_phase_ratio: 2", "return_phase_ratio: .nan", "no_action_penalty: .inf", "outward_penalty_scale: -1", "road: {width_threshold_m: 0}", "road: {highway: {weight: -1}}", "road: {surface: {other: .nan}}", "road: {typo: 1}"} {
		text := validYAML + "qlearning:\n  reward:\n    " + field + "\n"
		if _, err := loadYAML(t, text); err == nil {
			t.Fatalf("invalid reward accepted: %s", field)
		}
	}
	for _, scenario := range []string{"", ", scenario: D"} {
		text := validYAML + "qlearning:\n  training:\n    profiles: [{name: missing" + scenario + ", episodes: 3}]\n"
		if _, err := loadYAML(t, text); err == nil || !strings.Contains(err.Error(), "scenario") {
			t.Fatalf("invalid scenario accepted: %s %v", text, err)
		}
	}
}
