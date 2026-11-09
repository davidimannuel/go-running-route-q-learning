package qlearning

import "fmt"

// AttributeScoreConfig maps an OSM category to a signed research preference.
// Weights are used directly; scores are never clipped or normalized.
type AttributeScoreConfig struct {
	Weight   float64 `yaml:"weight" json:"weight"`
	Priority float64 `yaml:"priority" json:"priority"`
	Other    float64 `yaml:"other" json:"other"`
	Missing  float64 `yaml:"missing" json:"missing"`
}

// HighwayScoreConfig adds a preference for highway=secondary only.
// A missing secondary_priority keeps the legacy Other score; zero is valid.
type HighwayScoreConfig struct {
	AttributeScoreConfig `yaml:",inline"`
	SecondaryPriority    *float64 `yaml:"secondary_priority,omitempty" json:"secondary_priority,omitempty"`
}

func (c HighwayScoreConfig) secondaryScore() float64 {
	if c.SecondaryPriority != nil {
		return *c.SecondaryPriority
	}
	return c.Other
}

type RoadScoreConfig struct {
	Highway         HighwayScoreConfig   `yaml:"highway" json:"highway"`
	Surface         AttributeScoreConfig `yaml:"surface" json:"surface"`
	Width           AttributeScoreConfig `yaml:"width" json:"width"`
	WidthThresholdM float64              `yaml:"width_threshold_m" json:"width_threshold_m"`
}

// RewardConfig contains every numeric road and journey reward parameter.
// Defaults follow the reference; changing YAML starts a different experiment.
type RewardConfig struct {
	Road                     RoadScoreConfig `yaml:"road" json:"road"`
	ReturnPhaseRatio         float64         `yaml:"return_phase_ratio" json:"return_phase_ratio"`
	OutwardPenaltyScale      float64         `yaml:"outward_penalty_scale" json:"outward_penalty_scale"`
	ReturnRewardScale        float64         `yaml:"return_reward_scale" json:"return_reward_scale"`
	PermissivePenaltyPer100M float64         `yaml:"permissive_penalty_per_100m" json:"permissive_penalty_per_100m"`
	RevisitPenalty           float64         `yaml:"revisit_penalty" json:"revisit_penalty"`
	SuccessBonus             float64         `yaml:"success_bonus" json:"success_bonus"`
	OverDistancePenalty      float64         `yaml:"over_distance_penalty" json:"over_distance_penalty"`
	NoActionPenalty          float64         `yaml:"no_action_penalty" json:"no_action_penalty"`
	StepLimitPenalty         float64         `yaml:"step_limit_penalty" json:"step_limit_penalty"`
}

// DefaultRewardConfig preserves the agreed reference values when YAML omits a field.
func DefaultRewardConfig() RewardConfig {
	return RewardConfig{
		Road: RoadScoreConfig{
			Highway:         HighwayScoreConfig{AttributeScoreConfig: AttributeScoreConfig{Weight: 0.55, Priority: 1, Other: -1, Missing: -0.5}},
			Surface:         AttributeScoreConfig{Weight: 0.30, Priority: 1, Other: -1, Missing: -0.5},
			Width:           AttributeScoreConfig{Weight: 0.15, Priority: 1, Other: -1, Missing: -0.5},
			WidthThresholdM: 3,
		},
		ReturnPhaseRatio: 0.5, OutwardPenaltyScale: 0.6, ReturnRewardScale: 0.6,
		PermissivePenaltyPer100M: -0.1, RevisitPenalty: -10, SuccessBonus: 100,
		OverDistancePenalty: -50, NoActionPenalty: -50, StepLimitPenalty: -50,
	}
}

func (c RoadScoreConfig) Validate() error {
	for _, item := range []struct {
		name  string
		value AttributeScoreConfig
	}{
		{"highway", c.Highway.AttributeScoreConfig}, {"surface", c.Surface}, {"width", c.Width},
	} {
		v := item.value
		if !finite(v.Weight) || v.Weight < 0 || !finite(v.Priority) || !finite(v.Other) || !finite(v.Missing) {
			return fmt.Errorf("qlearning: reward.road.%s requires finite scores and a nonnegative finite weight", item.name)
		}
	}
	if !finite(c.Highway.secondaryScore()) {
		return fmt.Errorf("qlearning: reward.road.highway.secondary_priority must be finite")
	}
	total := c.Highway.Weight + c.Surface.Weight + c.Width.Weight
	if !finite(total) || total <= 0 || !finite(c.WidthThresholdM) || c.WidthThresholdM <= 0 {
		return fmt.Errorf("qlearning: reward.road requires positive finite total weight and width_threshold_m")
	}
	return nil
}

// Equal compares effective scores, not the addresses of optional config values.
func (c RewardConfig) Equal(other RewardConfig) bool {
	if c.Road.Highway.secondaryScore() != other.Road.Highway.secondaryScore() {
		return false
	}
	c.Road.Highway.SecondaryPriority = nil
	other.Road.Highway.SecondaryPriority = nil
	return c == other
}

func (c RewardConfig) Validate() error {
	if err := c.Road.Validate(); err != nil {
		return err
	}
	if !finite(c.ReturnPhaseRatio) || c.ReturnPhaseRatio < 0 || c.ReturnPhaseRatio > 1 {
		return fmt.Errorf("qlearning: reward.return_phase_ratio must be in [0,1]")
	}
	if !finite(c.OutwardPenaltyScale) || c.OutwardPenaltyScale < 0 || !finite(c.ReturnRewardScale) || c.ReturnRewardScale < 0 {
		return fmt.Errorf("qlearning: direction scales must be nonnegative and finite")
	}
	for _, v := range []float64{c.PermissivePenaltyPer100M, c.RevisitPenalty, c.SuccessBonus, c.OverDistancePenalty, c.NoActionPenalty, c.StepLimitPenalty} {
		if !finite(v) {
			return fmt.Errorf("qlearning: reward bonuses and penalties must be finite")
		}
	}
	return nil
}

// RewardComponents records values from the actual transition, not a second
// calculation for reporting. NoAction is separate to avoid double penalizing
// a dead end reached after moving. Closing to start never receives Revisit.
type RewardComponents struct {
	Edge, Access, Direction, Revisit, Terminal, NoAction float64
}

func (r RewardComponents) Total() float64 {
	return r.Edge + r.Access + r.Revisit + r.Terminal + r.Direction + r.NoAction
}
