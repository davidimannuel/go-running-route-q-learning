package qlearning

import (
	"fmt"
	"math"
)

// Scenario changes only the Q-table state, never the reward or action mask.
type Scenario string

const (
	// ScenarioA observes only the current node.
	ScenarioA Scenario = "A"
	// ScenarioB adds the distance-progress bin to the current node.
	ScenarioB Scenario = "B"
	// ScenarioC adds the previous node and distance-progress bin.
	ScenarioC Scenario = "C"
)

// Config contains shared settings and optional training profiles for one workdir.
// Reward constants intentionally remain in reward.go.
type Config struct {
	Scenario    Scenario          `yaml:"scenario" json:"scenario"`
	Environment EnvironmentConfig `yaml:"environment" json:"environment"`
	Training    TrainingConfig    `yaml:"training" json:"training"`
}

type EnvironmentConfig struct {
	TargetDistanceM   float64 `yaml:"target_distance_m" json:"target_distance_m"`
	DistanceTolerance float64 `yaml:"distance_tolerance" json:"distance_tolerance"`
	ProgressBins      int     `yaml:"progress_bins" json:"progress_bins"`
	MaxSteps          int     `yaml:"max_steps" json:"max_steps"`
	AllowRevisit      bool    `yaml:"allow_revisit" json:"allow_revisit"`
	RevisitAfterRatio float64 `yaml:"revisit_after_ratio" json:"revisit_after_ratio"`
	MaxNodeVisits     int     `yaml:"max_node_visits" json:"max_node_visits"`
	MaxSnapDistanceM  float64 `yaml:"max_snap_distance_m" json:"max_snap_distance_m"`
}

// TrainingProfile varies episode count and epsilon horizon while sharing graph,
// state, reward, hyperparameters, and seeds with the other profiles in a workdir.
// Name is also the result directory name; only lowercase letters, digits, and
// interior hyphens are accepted, preventing ambiguous or escaping output paths.
type TrainingProfile struct {
	Name                 string `yaml:"name" json:"name"`
	Episodes             int    `yaml:"episodes" json:"episodes"`
	EpsilonDecayEpisodes *int   `yaml:"epsilon_decay_episodes" json:"epsilon_decay_episodes"`
}

func (p TrainingProfile) Validate() error {
	if len(p.Name) == 0 || len(p.Name) > 64 || p.Name[0] == '-' || p.Name[len(p.Name)-1] == '-' {
		return fmt.Errorf("qlearning: profile name must have 1-64 lowercase letters/digits with optional interior hyphens")
	}
	for _, c := range p.Name {
		if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '-' {
			return fmt.Errorf("qlearning: invalid profile name %q", p.Name)
		}
	}
	if p.Episodes <= 0 || (p.EpsilonDecayEpisodes != nil && *p.EpsilonDecayEpisodes <= 0) {
		return fmt.Errorf("qlearning: profile %q requires positive episodes and a null or positive epsilon_decay_episodes", p.Name)
	}
	return nil
}

type TrainingConfig struct {
	// Nil uses the flat episodes/horizon settings for a simple single training.
	// Profiles override those durations; all other settings remain shared.
	Profiles []TrainingProfile `yaml:"profiles" json:"profiles,omitempty"`
	Episodes int               `yaml:"episodes" json:"episodes"`
	// Nil follows the total episode count; a supplied horizon must be positive.
	EpsilonDecayEpisodes *int    `yaml:"epsilon_decay_episodes" json:"epsilon_decay_episodes"`
	Alpha                float64 `yaml:"alpha" json:"alpha"`
	Gamma                float64 `yaml:"gamma" json:"gamma"`
	EpsilonStart         float64 `yaml:"epsilon_start" json:"epsilon_start"`
	EpsilonEnd           float64 `yaml:"epsilon_end" json:"epsilon_end"`
	// Nil seeds select automatic seeds. An explicitly empty list is invalid.
	Seeds      []uint64 `yaml:"seeds" json:"seeds"`
	Runs       *int     `yaml:"runs" json:"runs"`
	MaxWorkers int      `yaml:"max_workers" json:"max_workers"`
}

func DefaultConfig() Config {
	return Config{
		Environment: EnvironmentConfig{TargetDistanceM: 5000, DistanceTolerance: 0.10,
			ProgressBins: 16, MaxSteps: 220, AllowRevisit: true, RevisitAfterRatio: 0.50,
			MaxNodeVisits: 2, MaxSnapDistanceM: 100},
		Training: TrainingConfig{Episodes: 10000, Alpha: 0.15, Gamma: 0.95,
			EpsilonStart: 1, EpsilonEnd: 0.05, MaxWorkers: 1},
	}
}

func (s Scenario) Validate() error {
	if s != ScenarioA && s != ScenarioB && s != ScenarioC {
		return fmt.Errorf("qlearning: scenario must be A, B, or C")
	}
	return nil
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func (c EnvironmentConfig) MinDistanceM() float64 {
	return c.TargetDistanceM * (1 - c.DistanceTolerance)
}
func (c EnvironmentConfig) MaxDistanceM() float64 {
	return c.TargetDistanceM * (1 + c.DistanceTolerance)
}

func (c EnvironmentConfig) Validate() error {
	if !finite(c.TargetDistanceM) || c.TargetDistanceM <= 0 ||
		!finite(c.DistanceTolerance) || c.DistanceTolerance < 0 || c.DistanceTolerance >= 1 || !finite(c.MaxDistanceM()) {
		return fmt.Errorf("qlearning: target_distance_m must be positive and finite; distance_tolerance must be in [0,1)")
	}
	if c.ProgressBins <= 0 || c.MaxSteps <= 0 || c.MaxNodeVisits < 1 {
		return fmt.Errorf("qlearning: progress_bins, max_steps, and max_node_visits must be positive")
	}
	if !finite(c.RevisitAfterRatio) || c.RevisitAfterRatio < 0 || c.RevisitAfterRatio > 1 {
		return fmt.Errorf("qlearning: revisit_after_ratio must be in [0,1]")
	}
	if !finite(c.MaxSnapDistanceM) || c.MaxSnapDistanceM < 0 {
		return fmt.Errorf("qlearning: max_snap_distance_m must be nonnegative and finite")
	}
	return nil
}

func (c TrainingConfig) Validate() error {
	if c.Profiles != nil && len(c.Profiles) == 0 {
		return fmt.Errorf("qlearning: profiles must not be empty")
	}
	names := make(map[string]bool, len(c.Profiles))
	for _, profile := range c.Profiles {
		if err := profile.Validate(); err != nil {
			return err
		}
		if names[profile.Name] {
			return fmt.Errorf("qlearning: duplicate profile name %q", profile.Name)
		}
		names[profile.Name] = true
	}

	if c.Episodes <= 0 || c.MaxWorkers <= 0 {
		return fmt.Errorf("qlearning: episodes and max_workers must be positive")
	}
	if c.EpsilonDecayEpisodes != nil && *c.EpsilonDecayEpisodes <= 0 {
		return fmt.Errorf("qlearning: epsilon_decay_episodes must be null or positive")
	}
	if !finite(c.Alpha) || c.Alpha <= 0 || c.Alpha > 1 || !finite(c.Gamma) || c.Gamma < 0 || c.Gamma > 1 {
		return fmt.Errorf("qlearning: alpha must be in (0,1] and gamma in [0,1]")
	}
	if !finite(c.EpsilonStart) || !finite(c.EpsilonEnd) || c.EpsilonEnd < 0 || c.EpsilonStart > 1 || c.EpsilonEnd > c.EpsilonStart {
		return fmt.Errorf("qlearning: require 0 <= epsilon_end <= epsilon_start <= 1")
	}
	if c.Runs != nil && (*c.Runs <= 0 || c.Seeds != nil) {
		return fmt.Errorf("qlearning: runs must be positive and cannot be combined with seeds")
	}
	if c.Seeds != nil && len(c.Seeds) == 0 {
		return fmt.Errorf("qlearning: seeds must not be empty")
	}
	seen := make(map[uint64]bool, len(c.Seeds))
	for _, seed := range c.Seeds {
		if seed > MaxSeed || seen[seed] {
			return fmt.Errorf("qlearning: seeds must be distinct and at most %d", MaxSeed)
		}
		seen[seed] = true
	}
	return nil
}

func (c Config) Validate() error {
	if err := c.Scenario.Validate(); err != nil {
		return err
	}
	if err := c.Environment.Validate(); err != nil {
		return err
	}
	return c.Training.Validate()
}
