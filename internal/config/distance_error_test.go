package config

import "testing"

func TestDistanceErrorPenaltyConfigDefaultsAndZero(t *testing.T) {
	for _, tc := range []struct {
		yaml string
		want float64
	}{
		{"", 0}, {"  reward:\n    distance_error_penalty_per_100m: -5\n", -5},
		{"  reward:\n    distance_error_penalty_per_100m: 0\n", 0},
	} {
		cfg, err := loadYAML(t, validYAML+"qlearning:\n  scenario: B\n"+tc.yaml)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.QLearning.Reward.DistanceErrorPenaltyPer100M != tc.want {
			t.Fatalf("reward=%+v", cfg.QLearning.Reward)
		}
	}
}
