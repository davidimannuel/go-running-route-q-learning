package qlearning

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"

	"github.com/davidimannuel/go-running-route-q-learning/internal/logger"
)

// Reserve room for the reference evaluation seed offset without uint64 wrap.
const EvaluationSeedOffset uint64 = 100000
const MaxSeed = ^uint64(0) - EvaluationSeedOffset

// ResolveSeeds is called once per command. Persist its result before training;
// omission selects one automatic seed, or Runs distinct automatic seeds.
func ResolveSeeds(cfg TrainingConfig) ([]uint64, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.Seeds != nil {
		return append([]uint64(nil), cfg.Seeds...), nil
	}
	count := 1
	if cfg.Runs != nil {
		count = *cfg.Runs
	}
	seeds := make([]uint64, 0, count)
	seen := make(map[uint64]bool, count)
	for len(seeds) < count {
		seed := rand.Uint64N(MaxSeed + 1)
		if !seen[seed] {
			seeds = append(seeds, seed)
			seen[seed] = true
		}
	}
	return seeds, nil
}

// EpsilonForEpisode receives a zero-based episode index. A nil horizon follows
// total episodes; a fixed horizon holds epsilon_end after its last episode.
func EpsilonForEpisode(episode int, cfg TrainingConfig) float64 {
	horizon := cfg.Episodes
	if cfg.EpsilonDecayEpisodes != nil {
		horizon = *cfg.EpsilonDecayEpisodes
	}
	progress := math.Min(1, float64(episode)/float64(max(1, horizon-1)))
	return cfg.EpsilonEnd + (cfg.EpsilonStart-cfg.EpsilonEnd)*(1-progress)
}

type TrainingResult struct {
	Table      QTable
	Evaluation Metrics
	// Only the short final greedy route is retained; histories stream to the caller.
	EvaluationSteps []StepResult
}

// Train starts a fresh Q-table/RNG for one resolved profile and seed.
// Select a profile first (Profiles must be nil). Reset never reseeds the RNG.
// The callback runs once per episode and can stream to a bounded writer/channel.
func Train(ctx context.Context, p *PreparedGraph, cfg Config, seed uint64, history func(Metrics) error) (TrainingResult, error) {
	if cfg.Training.Profiles != nil {
		return TrainingResult{}, fmt.Errorf("qlearning: resolve one training profile before calling Train")
	}
	if err := cfg.Validate(); err != nil {
		return TrainingResult{}, err
	}
	if seed > MaxSeed {
		return TrainingResult{}, fmt.Errorf("qlearning: seed exceeds %d", MaxSeed)
	}
	if p == nil || !p.reward.Equal(cfg.Reward) {
		return TrainingResult{}, fmt.Errorf("qlearning: prepared graph reward does not match effective config")
	}
	env, err := NewEnvironment(p, cfg.Scenario, cfg.Environment)
	if err != nil {
		return TrainingResult{}, err
	}
	rng := rand.New(rand.NewPCG(seed, 0))
	table := make(QTable)
	logger.Debug(ctx, "Seed training started", logger.String("scenario", string(cfg.Scenario)), logger.Uint64("seed", seed), logger.Int("episodes", cfg.Training.Episodes))
	interval := max(1, cfg.Training.Episodes/10)
	for episode := 0; episode < cfg.Training.Episodes; episode++ {
		if err := ctx.Err(); err != nil {
			return TrainingResult{}, err
		}
		env.Reset()
		epsilon := EpsilonForEpisode(episode, cfg.Training)
		reward, tdSum := 0.0, 0.0
		updates := 0
		for !env.done {
			if err := ctx.Err(); err != nil {
				return TrainingResult{}, err
			}
			step, err := TrainStep(env, table, rng, cfg.Training, epsilon, nil)
			if err != nil {
				return TrainingResult{}, err
			}
			reward += step.RewardStep
			reward += step.StopPenalty
			if step.QUpdated {
				tdSum += math.Abs(step.TDError)
				updates++
			}
		}
		metrics := env.Metrics(reward)
		metrics.Seed, metrics.Episode, metrics.Epsilon, metrics.QStateCount = seed, episode+1, epsilon, len(table)
		if updates > 0 {
			metrics.MeanAbsTDError = tdSum / float64(updates)
		}
		if history != nil {
			if err := history(metrics); err != nil {
				return TrainingResult{}, err
			}
		}
		if (episode+1)%interval == 0 || episode+1 == cfg.Training.Episodes {
			logger.Debug(ctx, "Training progress", logger.Uint64("seed", seed), logger.Int("episode", episode+1), logger.Int("episodes", cfg.Training.Episodes), logger.Float64("epsilon", epsilon), logger.Int("q_states", len(table)))
		}
	}
	metrics, steps, err := Evaluate(ctx, p, cfg, seed, table)
	if err != nil {
		return TrainingResult{}, err
	}
	logger.Debug(ctx, "Greedy evaluation completed", logger.Uint64("seed", seed), logger.Bool("is_loop", metrics.IsLoop), logger.Float64("distance_m", metrics.TotalDistanceM), logger.String("termination", string(metrics.TerminationReason)))
	return TrainingResult{Table: table, Evaluation: metrics, EvaluationSteps: steps}, nil
}

// Evaluate uses epsilon=0 and seed+100000. Near-equal greedy ties still use RNG.
// Numeric Q-values are never updated. As in the reference defaultdict, policy
// lookups can create empty state entries, which are included in q_state_count.
func Evaluate(ctx context.Context, p *PreparedGraph, cfg Config, seed uint64, table QTable) (Metrics, []StepResult, error) {
	if cfg.Training.Profiles != nil {
		return Metrics{}, nil, fmt.Errorf("qlearning: use a resolved profile configuration for Evaluate")
	}
	if err := cfg.Validate(); err != nil {
		return Metrics{}, nil, err
	}
	if table == nil || seed > MaxSeed {
		return Metrics{}, nil, fmt.Errorf("evaluate: valid seed and non-nil Q-table required")
	}
	if p == nil || !p.reward.Equal(cfg.Reward) {
		return Metrics{}, nil, fmt.Errorf("qlearning: prepared graph reward does not match effective config")
	}
	env, err := NewEnvironment(p, cfg.Scenario, cfg.Environment)
	if err != nil {
		return Metrics{}, nil, err
	}
	rng := rand.New(rand.NewPCG(seed+EvaluationSeedOffset, 0))
	steps := make([]StepResult, 0, cfg.Environment.MaxSteps)
	reward := 0.0
	for !env.done {
		if err := ctx.Err(); err != nil {
			return Metrics{}, nil, err
		}
		var selected *ActionID
		if id, ok := chooseAction(env, table, rng, 0); ok {
			selected = &id
		}
		step, err := advance(env, selected)
		if err != nil {
			return Metrics{}, nil, err
		}
		// Include Q diagnostics in the final trace without changing the table.
		if step.ActionTaken {
			step.QBefore = table.Value(step.State, step.Action)
			step.QAfter = step.QBefore
		}
		reward += step.RewardStep
		reward += step.StopPenalty
		steps = append(steps, step)
	}
	metrics := env.Metrics(reward)
	metrics.Seed = seed
	metrics.QStateCount = len(table)
	return metrics, steps, nil
}
