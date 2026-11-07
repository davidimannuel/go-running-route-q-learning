package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidimannuel/go-running-route-q-learning/internal/analysis"
	"github.com/davidimannuel/go-running-route-q-learning/internal/config"
	"github.com/davidimannuel/go-running-route-q-learning/internal/logger"
	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
	"github.com/davidimannuel/go-running-route-q-learning/internal/qlearning"
)

// runTrain never downloads or silently modifies a graph. --snapshot lets all
// matched workdirs use exactly one previously inspected local OSM snapshot.
func runTrain(ctx context.Context, args []string, out, logOut io.Writer) error {
	flags := flag.NewFlagSet("train", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	workdirFlag := flags.String("workdir", "", "Directory containing config.yaml")
	snapshotFlag := flags.String("snapshot", "", "Use this local osm.json instead of output_dir/osm.json")
	profileFlag := flags.String("profile", "", "Run only the named training profile; default: all configured profiles")
	debug := flags.Bool("debug", false, "Include debug logs on stderr")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(out)
			return nil
		}
		return fmt.Errorf("train: %w", err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("train: unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if strings.TrimSpace(*workdirFlag) == "" {
		return fmt.Errorf("train: --workdir is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ctx = logger.ContextWithLogger(ctx, logger.New(logOut, *debug))
	workdir, err := filepath.Abs(*workdirFlag)
	if err != nil {
		return err
	}
	cfg, err := config.Load(workdir)
	if err != nil {
		return fmt.Errorf("train: %w", err)
	}
	if err := cfg.QLearning.Validate(); err != nil {
		return fmt.Errorf("train: %w; configure qlearning in config.yaml", err)
	}
	profiles, err := selectTrainingProfiles(cfg.QLearning.Training, *profileFlag)
	if err != nil {
		return err
	}
	output := filepath.Join(workdir, cfg.OutputDir)
	snapshot := filepath.Join(output, "osm.json")
	if *snapshotFlag != "" {
		snapshot, err = filepath.Abs(*snapshotFlag)
		if err != nil {
			return err
		}
	}
	graph, hash, err := loadTrainingGraph(snapshot)
	if err != nil {
		return fmt.Errorf("train: load local snapshot (run download first, or use --snapshot): %w", err)
	}
	start, snapM, err := qlearning.NearestStart(graph, cfg.Map.Latitude, cfg.Map.Longitude)
	if err != nil {
		return err
	}
	if snapM > cfg.QLearning.Environment.MaxSnapDistanceM {
		return fmt.Errorf("train: nearest start node %d is %.2f m away, exceeding max_snap_distance_m %.2f", start, snapM, cfg.QLearning.Environment.MaxSnapDistanceM)
	}
	prepared, err := qlearning.NewPreparedGraph(graph, start, cfg.QLearning.Reward)
	if err != nil {
		return err
	}
	if !prepared.HasStartActions() {
		return fmt.Errorf("train: nearest start node %d has no access-allowed positive-length actions", start)
	}
	seeds, err := qlearning.ResolveSeeds(cfg.QLearning.Training)
	if err != nil {
		return err
	}
	logger.Debug(ctx, "Training graph prepared", logger.Int("nodes", len(graph.Nodes)), logger.Int("directed_edges", graph.EdgeCount()), logger.Int("start_node", start), logger.Float64("snap_distance_m", snapM))
	logger.Debug(ctx, "Graph uses raw unsimplified segments and Haversine start distances; results are not numerically equivalent to the projected, simplified Python graph")
	logger.Debug(ctx, "Training seeds resolved", logger.String("seeds", fmt.Sprint(seeds)), logger.Int("workers", min(len(seeds), cfg.QLearning.Training.MaxWorkers)))
	metadata := trainingMetadata(cfg, snapshot, hash, start, snapM, graph, seeds)
	metadata["config_path"] = filepath.Join(workdir, config.Filename)
	if profiles == nil {
		return executeTraining(ctx, out, prepared, cfg, seeds, output, metadata)
	}
	return executeProfiles(ctx, out, prepared, cfg, seeds, output, metadata, profiles)
}

func loadTrainingGraph(path string) (*osm.Graph, string, error) {
	// Decode and validate exactly the bytes whose hash is recorded. This avoids
	// a second full snapshot read and ensures the hash identifies the trained data.
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	data, err := osm.Parse(body)
	if err != nil {
		return nil, "", err
	}

	graph := osm.NewGraph()
	if err := graph.LoadOSM(data); err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(body)
	return graph, hex.EncodeToString(sum[:]), nil
}

func trainingMetadata(cfg config.Config, snapshot, hash string, start int, snapM float64, graph *osm.Graph, seeds []uint64) map[string]any {
	return map[string]any{
		"status": "running", "started_at": time.Now().UTC().Format(time.RFC3339Nano),
		"config": cfg.QLearning, "workdir_config": cfg, "map": cfg.Map, "resolved_seeds": seeds,
		"schema_version": 1, "build": buildMetadata(),
		"automatic_seeds": cfg.QLearning.Training.Seeds == nil,
		"snapshot_path":   snapshot, "snapshot_sha256": hash, "nodes": len(graph.Nodes), "directed_edges": graph.EdgeCount(),
		"start_node": start, "snap_distance_m": snapM, "graph_simplified": false,
		"start_selection":        "nearest Haversine node; ties by smaller node ID",
		"start_distance_method":  "Haversine; mean Earth radius 6371009 m; precomputed once per node",
		"rng":                    "math/rand/v2 PCG(seed, 0); one independent RNG per seed; not NumPy-equivalent",
		"evaluation_seed_offset": qlearning.EvaluationSeedOffset, "evaluation_epsilon": 0,
		"actual_workers":      min(len(seeds), cfg.QLearning.Training.MaxWorkers),
		"reward_scheme":       "configurable_signed_unclipped_v1",
		"reward_constants":    cfg.QLearning.Reward,
		"road_score":          cfg.QLearning.Reward.Road,
		"access_policy":       "single normalized value: blank, yes, permissive; foot does not override; combined values rejected",
		"action_identity":     "(next_node, key); sorted numerically by (next_node, key)",
		"action_mask":         "fresh + permitted node revisits + valid closing; shared by exploration, greedy, and bootstrap",
		"state_fields":        map[string][]string{"A": {"current_node"}, "B": {"current_node", "progress_bin"}, "C": {"current_node", "previous_node", "progress_bin"}},
		"reset_previous_node": -1, "q_formula": "Q <- Q + alpha * (reward + gamma * max_legal_next_Q - Q); terminal bootstrap=0",
		"epsilon_formula":          "end + (start-end)*(1-min(episode_zero_based/max(1,horizon-1),1)); null horizon follows total episodes",
		"tie_break":                "uniform among np.isclose-equivalent values; rtol=1e-5, atol=1e-8",
		"overlap_basis":            "physical_road_traversal_count",
		"overlap_formula":          "(total traversals - unique physical roads) / total traversals; NaN if empty",
		"physical_road_identity":   "canonical straight-segment endpoint coordinates; reverse/identical parallel geometry share identity",
		"representative_selection": "valid loops first; smallest absolute distance error, then higher comfort/return progress, then seed; failed ties also use end distance",
		"reference":                "rute_lari_core.py and scenario_A/B/C.ipynb (active November 2026 rules)",
	}
}

// executeTraining stages the entire new result. Failure/cancellation retains the
// previous completed directory; training-run.json explicitly records the latest
// attempt's status and actual automatic seeds, even when that attempt fails.
func executeTraining(ctx context.Context, out io.Writer, p *qlearning.PreparedGraph, cfg config.Config, seeds []uint64, output string, metadata map[string]any) error {
	return executeTrainingAt(ctx, out, p, cfg, seeds, filepath.Join(output, "training"), filepath.Join(output, "training-run.json"), metadata)
}

func executeTrainingAt(ctx context.Context, out io.Writer, p *qlearning.PreparedGraph, cfg config.Config, seeds []uint64, target, marker string, metadata map[string]any) (runErr error) {
	started := time.Now()
	if err := writeTrainingHeading(out, cfg, seeds, metadata); err != nil {
		return err
	}
	directory := filepath.Dir(target)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
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
	stage, err := os.MkdirTemp(directory, ".training-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	logger.Debug(ctx, "Training started", logger.String("scenario", string(cfg.QLearning.Scenario)), logger.String("status_path", marker))
	trials, err := trainBatch(ctx, out, p, cfg.QLearning, seeds, stage)
	if err != nil {
		return fmt.Errorf("train: %w", err)
	}
	if err := finishTrainingFiles(ctx, p, cfg, trials, seeds, stage); err != nil {
		return err
	}
	metadata["status"], metadata["completed_at"] = "completed", time.Now().UTC().Format(time.RFC3339Nano)
	rows := make([]qlearning.Metrics, len(trials))
	for i, t := range trials {
		rows[i] = t.Metrics
	}
	metadata["representative_seed"] = seeds[qlearning.Representative(rows)]
	metadata["artifacts"] = []string{"history.csv", "evaluation.csv", "routes.html", "report.html", "metadata.json", "seed-<seed>/qtable.csv", "seed-<seed>/evaluation-steps.csv", "seed-<seed>/metadata.json"}
	evaluations := make([]map[string]any, 0, len(trials))
	for _, trial := range trials {
		evaluations = append(evaluations, evaluationMetadata(trial.Metrics))
	}
	metadata["evaluations"] = evaluations
	for i, seed := range seeds {
		detail := make(map[string]any, len(metadata))
		for key, value := range metadata {
			if key != "evaluations" {
				detail[key] = value
			}
		}
		detail["seed"] = fmt.Sprint(seed)
		detail["evaluation"] = evaluations[i]
		detail["artifacts"] = []string{"qtable.csv", "evaluation-steps.csv", "metadata.json"}
		if err := writeJSON(ctx, filepath.Join(stage, fmt.Sprintf("seed-%d", seed), "metadata.json"), detail); err != nil {
			return err
		}
	}
	if err := writeOutput(ctx, filepath.Join(stage, "report.html"), func(w io.Writer) error { return analysis.ReportToHTML(w, "Training details", "training", metadata) }); err != nil {
		return err
	}
	if err := writeJSON(ctx, filepath.Join(stage, "metadata.json"), metadata); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := publishTraining(stage, target); err != nil {
		return err
	}
	if err := writeJSON(context.WithoutCancel(ctx), marker, metadata); err != nil {
		return err
	}
	logger.Debug(ctx, "Training completed", logger.String("output_dir", target), logger.String("map", filepath.Join(target, "routes.html")))
	if err := writeTrainingSummary(out, trials, time.Since(started), target); err != nil {
		return err
	}
	return nil
}

func writeJSON(ctx context.Context, path string, value any) error {
	return writeOutput(ctx, path, func(w io.Writer) error {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	})
}

// publishTraining replaces only the generated training directory. Roll back the
// previous result if publishing the new directory fails; map/analysis stay intact.
func publishTraining(stage, target string) error {
	backup := stage + "-previous"
	moved := false
	if _, err := os.Lstat(target); err == nil {
		if err := os.Rename(target, backup); err != nil {
			return fmt.Errorf("backup previous training: %w", err)
		}
		moved = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(stage, target); err != nil {
		if moved {
			if restoreErr := os.Rename(backup, target); restoreErr != nil {
				return errors.Join(err, restoreErr)
			}
		}
		return fmt.Errorf("publish training: %w", err)
	}
	if moved {
		if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("remove previous training backup: %w", err)
		}
	}
	return nil
}
