package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"runtime/debug"

	"github.com/davidimannuel/go-running-route-q-learning/internal/qlearning"
)

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func buildMetadata() map[string]any {
	result := map[string]any{"go_version": runtime.Version(), "platform": runtime.GOOS + "/" + runtime.GOARCH}
	if info, ok := debug.ReadBuildInfo(); ok {
		result["module"] = info.Main.Path
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" || setting.Key == "vcs.modified" || setting.Key == "vcs.time" {
				result[setting.Key] = setting.Value
			}
		}
	}
	return result
}

func evaluationMetadata(m qlearning.Metrics) map[string]any {
	finite := func(v float64) any {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil
		}
		return v
	}
	return map[string]any{"scenario": m.Scenario, "seed": fmt.Sprint(m.Seed), "is_loop": m.IsLoop,
		"termination_reason": m.TerminationReason, "total_distance_m": m.TotalDistanceM,
		"absolute_distance_error_m": m.AbsoluteDistanceErrorM, "total_reward": m.TotalReward,
		"mean_comfort": finite(m.MeanComfort), "overlap_ratio": finite(m.OverlapRatio),
		"return_progress_ratio": finite(m.ReturnProgressRatio), "distance_to_start_at_end_m": m.DistanceToStartAtEndM,
		"closing_action_available": m.ClosingActionAvailable, "steps": m.Steps, "revisit_count": m.RevisitCount, "q_state_count": m.QStateCount}
}
