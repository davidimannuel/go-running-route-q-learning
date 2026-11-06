package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/davidimannuel/go-running-route-q-learning/internal/config"
	"github.com/davidimannuel/go-running-route-q-learning/internal/logger"
	"github.com/davidimannuel/go-running-route-q-learning/internal/qlearning"
)

func selectTrainingProfiles(cfg qlearning.TrainingConfig, name string) ([]qlearning.TrainingProfile, error) {
	if name == "" {
		return cfg.Profiles, nil
	}
	for _, profile := range cfg.Profiles {
		if profile.Name == name {
			return []qlearning.TrainingProfile{profile}, nil
		}
	}
	return nil, fmt.Errorf("train: unknown profile %q; use a name from qlearning.training.profiles", name)
}

type profileStatus struct {
	Name                 string `json:"name"`
	Episodes             int    `json:"episodes"`
	EpsilonDecayEpisodes *int   `json:"epsilon_decay_episodes"`
	Status               string `json:"status"`
}

// executeProfiles reuses one prepared graph and one resolved seed list. Profiles
// run sequentially, with independent seed workers inside each profile. This
// bounds memory and keeps state comparisons matched; no Q-table is continued
// from a shorter profile or shared between profiles.
func executeProfiles(ctx context.Context, out io.Writer, p *qlearning.PreparedGraph, cfg config.Config, seeds []uint64, output string, metadata map[string]any, profiles []qlearning.TrainingProfile) (runErr error) {
	marker := filepath.Join(output, "training-run.json")
	statuses := make([]profileStatus, len(profiles))
	for i, profile := range profiles {
		statuses[i] = profileStatus{profile.Name, profile.Episodes, profile.EpsilonDecayEpisodes, "pending"}
	}
	metadata["profiles"] = statuses
	if err := writeJSON(ctx, marker, metadata); err != nil {
		return err
	}
	defer func() {
		if runErr != nil {
			metadata["status"], metadata["error"] = "failed", runErr.Error()
			if err := writeJSON(context.WithoutCancel(ctx), marker, metadata); err != nil {
				runErr = errors.Join(runErr, err)
			}
		}
	}()
	for i, profile := range profiles {
		if err := ctx.Err(); err != nil {
			return err
		}
		statuses[i].Status = "running"
		if err := writeJSON(ctx, marker, metadata); err != nil {
			return err
		}
		effective := cfg
		effective.QLearning.Training.Profiles = nil
		effective.QLearning.Training.Episodes = profile.Episodes
		effective.QLearning.Training.EpsilonDecayEpisodes = profile.EpsilonDecayEpisodes
		// Construct new per-profile metadata without inheriting a completed status,
		// representative seed, or another profile's effective training duration.
		details := make(map[string]any, len(metadata))
		for key, value := range metadata {
			if key != "profiles" {
				details[key] = value
			}
		}
		details["config"] = effective.QLearning
		details["training_profile"] = profile.Name
		details["started_at"] = time.Now().UTC().Format(time.RFC3339Nano)
		target := filepath.Join(output, "training", profile.Name)
		profileMarker := filepath.Join(output, "training", profile.Name+"-run.json")
		logger.Debug(ctx, "Training profile started", logger.String("profile", profile.Name), logger.Int("episodes", profile.Episodes), logger.Int("profile_index", i+1), logger.Int("profile_count", len(profiles)))
		if err := executeTrainingAt(ctx, out, p, effective, seeds, target, profileMarker, details); err != nil {
			statuses[i].Status = "failed"
			return fmt.Errorf("profile %s: %w", profile.Name, err)
		}
		statuses[i].Status = "completed"
		if err := writeJSON(ctx, marker, metadata); err != nil {
			return err
		}
	}
	metadata["status"], metadata["completed_at"] = "completed", time.Now().UTC().Format(time.RFC3339Nano)
	if err := writeJSON(context.WithoutCancel(ctx), marker, metadata); err != nil {
		return err
	}
	logger.Debug(ctx, "All selected training profiles completed", logger.Int("profiles", len(profiles)), logger.String("output_dir", filepath.Join(output, "training")))
	return nil
}
