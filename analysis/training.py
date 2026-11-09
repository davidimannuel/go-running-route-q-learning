"""Report Go training artifacts; this module never trains or updates Q-values.

Chart layouts follow scenario_A/B/C.ipynb and rute_lari_core.py in the reference
project. Histories are read in chunks; seed IDs retain uint64 precision as text.
"""

from pathlib import Path
import json

import matplotlib.pyplot as plt
import pandas as pd
from IPython.display import display

from .common import PROJECT_ROOT, positive_integer, read_workdir, resolve_workdir, show_environment

TERMINATIONS = ["success", "over_distance", "no_available_action", "step_limit"]
TERMINATION_LABELS = ["Berhasil loop", "Melebihi 5,5 km", "Tidak ada action", "Batas langkah"]
TERMINATION_COLORS = ["#16a34a", "#dc2626", "#7c3aed", "#475569"]
MEAN_COLUMNS = [
    "is_loop", "total_distance_m", "distance_km", "absolute_distance_error_m",
    "total_reward", "mean_comfort", "q_state_count", "epsilon", "overlap_ratio",
    "distance_to_start_at_end_m", "return_progress_ratio", "closing_action_available",
    "mean_abs_td_error", "steps", "revisit_count",
]
EVALUATION_COLUMNS = [
    "scenario", "seed", "is_loop", "total_distance_m", "absolute_distance_error_m",
    "total_reward", "mean_comfort", "overlap_ratio", "return_progress_ratio",
    "distance_to_start_at_end_m", "closing_action_available", "termination_reason",
    "steps", "revisit_count", "q_state_count",
]
EVALUATION_DISPLAY_COLUMNS = EVALUATION_COLUMNS[2:EVALUATION_COLUMNS.index("termination_reason") + 1]


class TrainingAnalysis:
    """Load completed runs and display the same report sections as the notebooks."""

    def __init__(self, workdir: str | Path, *, profiles: list[str] | None = None,
                 rolling_window: int = 50, tail_episodes: int = 10_000,
                 plot_every: int = 1, chunk_size: int = 50_000):
        for name, value in (("rolling_window", rolling_window), ("tail_episodes", tail_episodes),
                            ("plot_every", plot_every), ("chunk_size", chunk_size)):
            positive_integer(name, value)
        self.workdir = resolve_workdir(workdir)
        self.profiles = None if profiles is None else list(profiles)
        self.rolling_window = rolling_window
        self.tail_episodes = tail_episodes
        self.plot_every = plot_every
        self.chunk_size = chunk_size
        self.results = []
        self.analysis_dir = None
        self.profile_summary = None
        self.last_window_summary = None

    def _require_loaded(self):
        if not self.results or self.analysis_dir is None:
            raise RuntimeError("Call load() before displaying training report sections.")

    def result_locations(self, workdir):
        workdir = resolve_workdir(workdir)
        config, output = read_workdir(workdir)
        profiles = config.get("qlearning", {}).get("training", {}).get("profiles")
        if profiles is None:
            return output, [("single", output / "training")]
        locations = []
        for profile in profiles:
            name = profile["name"]
            if not name or any(character not in "abcdefghijklmnopqrstuvwxyz0123456789-" for character in name):
                raise ValueError(f"Invalid profile name: {name}")
            locations.append((name, output / "training" / name))
        return output, locations

    def read_result(self, workdir, name, directory):
        metadata = json.loads((directory / "metadata.json").read_text())
        if metadata.get("status") != "completed":
            raise ValueError(f"Incomplete metadata: {directory}")
        marker = (directory.parent / "training-run.json" if name == "single"
                  else directory.parent / f"{name}-run.json")
        if marker.is_file():
            latest = json.loads(marker.read_text())
            if latest.get("status") != "completed" or latest.get("completed_at") != metadata.get("completed_at"):
                raise ValueError(f"Latest attempt is incomplete/failed; previous results retained: {directory}")
        seeds = [str(seed) for seed in metadata["resolved_seeds"]]
        if not seeds or len(set(seeds)) != len(seeds):
            raise ValueError(f"Invalid recorded seed list: {directory}")
        evaluation = pd.read_csv(directory / "evaluation.csv", usecols=EVALUATION_COLUMNS,
                                 dtype={"seed": "string", "scenario": "string",
                                        "is_loop": "bool", "closing_action_available": "bool"})
        if evaluation["seed"].duplicated().any() or set(evaluation["seed"]) != set(seeds):
            raise ValueError(f"Evaluation does not contain exactly one row per recorded seed: {directory}")
        scenario = metadata["config"]["scenario"]
        if not evaluation["scenario"].eq(scenario).all():
            raise ValueError(f"Evaluation scenario disagrees with metadata: {directory}")
        if not evaluation["termination_reason"].isin(TERMINATIONS).all():
            raise ValueError(f"Unknown termination reason: {directory}")
        if not evaluation["is_loop"].eq(evaluation["termination_reason"].eq("success")).all():
            raise ValueError(f"Loop/termination columns disagree: {directory}")
        return {"name": name, "directory": directory, "metadata": metadata, "seeds": seeds,
                "evaluation": evaluation, "scenario": scenario, "workdir": workdir}

    def discover_results(self, workdir, selected=None):
        workdir = resolve_workdir(workdir)
        output, locations = self.result_locations(workdir)
        names = {name for name, _ in locations}
        if selected is not None and set(selected) - names:
            raise ValueError(f"Unknown profiles: {set(selected) - names}")
        records, availability = [], []
        for name, directory in locations:
            if selected is not None and name not in selected:
                continue
            if not (directory / "metadata.json").is_file():
                availability.append({"workdir": workdir.name, "profile": name, "status": "belum tersedia"})
                continue
            record = self.read_result(workdir, name, directory)
            records.append(record)
            availability.append({"workdir": workdir.name, "profile": name, "status": "completed"})
        return output, records, availability

    def read_history(self, result):
        metadata = result["metadata"]
        training = metadata["config"]["training"]
        total_episodes = int(training["episodes"])
        index = pd.RangeIndex(1, total_episodes + 1, name="episode")
        sums = pd.DataFrame(0.0, index=index, columns=MEAN_COLUMNS)
        counts = pd.DataFrame(0, index=index, columns=MEAN_COLUMNS)
        reason_counts = pd.DataFrame(0, index=index, columns=TERMINATIONS)
        seeds = result["seeds"]
        tail_sums = pd.DataFrame(0.0, index=pd.Index(seeds, name="seed"), columns=MEAN_COLUMNS)
        tail_counts = pd.DataFrame(0, index=tail_sums.index, columns=MEAN_COLUMNS)
        last_episode = dict.fromkeys(seeds, 0)
        horizon = training["epsilon_decay_episodes"] or total_episodes
        required = ["scenario", "seed", "episode", "termination_reason", *MEAN_COLUMNS]
        dtypes = {column: "float64" for column in MEAN_COLUMNS}
        dtypes.update({"seed": "string", "scenario": "string", "episode": "int64",
                       "is_loop": "bool", "closing_action_available": "bool"})
        reader = pd.read_csv(result["directory"] / "history.csv", usecols=required,
                             dtype=dtypes, chunksize=self.chunk_size)
        for chunk in reader:
            if not chunk["scenario"].eq(result["scenario"]).all():
                raise ValueError("History scenario disagrees with metadata.")
            if not chunk["termination_reason"].isin(TERMINATIONS).all():
                raise ValueError("History contains an unknown termination reason.")
            if not chunk["is_loop"].eq(chunk["termination_reason"].eq("success")).all():
                raise ValueError("History loop/termination columns disagree.")
            for seed, rows in chunk.groupby("seed", sort=False):
                if seed not in last_episode or rows["episode"].iloc[0] != last_episode[seed] + 1:
                    raise ValueError(f"Missing/duplicate/unexpected seed episodes: {seed}")
                if rows["episode"].diff().dropna().ne(1).any() or rows["episode"].iloc[-1] > total_episodes:
                    raise ValueError(f"Noncontiguous or excessive episodes: {seed}")
                last_episode[seed] = int(rows["episode"].iloc[-1])
            progress = ((chunk["episode"] - 1) / max(1, horizon - 1)).clip(upper=1)
            expected_epsilon = training["epsilon_end"] + (training["epsilon_start"] - training["epsilon_end"]) * (1 - progress)
            if (chunk["epsilon"] - expected_epsilon).abs().gt(1e-12).any() or chunk["epsilon"].isna().any():
                raise ValueError("Stored epsilon does not match the recorded schedule.")
            if chunk[MEAN_COLUMNS].isin([float("inf"), -float("inf")]).any().any():
                raise ValueError("History contains infinite numeric values.")
            for column in ["overlap_ratio", "return_progress_ratio"]:
                if not chunk[column].dropna().between(0, 1).all():
                    raise ValueError(f"Invalid ratio: {column}")
            # Keep sums/counts separate: missing ratios must not become zero.
            grouped = chunk.groupby("episode")[MEAN_COLUMNS]
            partial_sum, partial_count = grouped.sum(), grouped.count()
            sums.loc[partial_sum.index] += partial_sum
            counts.loc[partial_count.index] += partial_count
            reasons = pd.crosstab(chunk["episode"], chunk["termination_reason"]).reindex(columns=TERMINATIONS, fill_value=0)
            reason_counts.loc[reasons.index] += reasons
            tail = chunk.loc[chunk["episode"] > max(0, total_episodes - self.tail_episodes)]
            grouped_tail = tail.groupby("seed")[MEAN_COLUMNS]
            tail_sum, tail_count = grouped_tail.sum(), grouped_tail.count()
            tail_sums.loc[tail_sum.index] += tail_sum
            tail_counts.loc[tail_count.index] += tail_count
        if any(last != total_episodes for last in last_episode.values()):
            raise ValueError("History is truncated or a recorded seed is missing.")
        if not counts["is_loop"].eq(len(seeds)).all():
            raise ValueError("History does not have one observation per seed/episode.")
        latest = self.read_result(result["workdir"], result["name"], result["directory"])
        if latest["metadata"] != metadata:
            raise ValueError("Training results changed during analysis; restart and run all cells.")
        means = sums / counts.replace(0, float("nan"))
        rolling = means.rolling(self.rolling_window, min_periods=1).mean()
        # Roll observations together, not equally weighted per-episode ratios.
        overlap_totals = sums["overlap_ratio"].rolling(self.rolling_window, min_periods=1).sum()
        overlap_counts = counts["overlap_ratio"].rolling(self.rolling_window, min_periods=1).sum()
        overlap = overlap_totals / overlap_counts.replace(0, float("nan"))
        termination = reason_counts / len(seeds)
        return {"means": means, "rolling": rolling, "overlap": overlap,
                "termination_rolling": termination.rolling(self.rolling_window, min_periods=1).mean(),
                "termination_counts": reason_counts.sum(),
                "tail_by_seed": tail_sums / tail_counts.replace(0, float("nan")),
                "tail_mean": tail_sums.sum() / tail_counts.sum().replace(0, float("nan")),
                "tail_episode_count": min(total_episodes, self.tail_episodes)}

    def termination_table(self, counts):
        counts = counts.reindex(TERMINATIONS, fill_value=0)
        total = counts.sum()
        return pd.DataFrame({"termination_reason": TERMINATIONS, "count": counts.to_numpy(),
                             "termination_rate": (counts / total).to_numpy(),
                             "termination_rate_percent": (counts / total * 100).to_numpy()})

    def sampled(self, frame):
        # Smoothing/aggregation happens before visual downsampling.
        positions = sorted(set([0, *range(self.plot_every - 1, len(frame), self.plot_every), len(frame) - 1]))
        return frame.iloc[positions]

    def save_figure(self, figure, directory, name):
        directory.mkdir(parents=True, exist_ok=True)
        figure.tight_layout()
        for extension in ("png", "svg"):
            figure.savefig(directory / f"{name}.{extension}", dpi=160, bbox_inches="tight")
        plt.show()
        plt.close(figure)

    def chart_title(self, results):
        """Label each plotted run with its scenario and profile name."""
        return "\n".join(f"Scenario {result['scenario']} | {result['name']}" for result in results)

    def plot_training(self, result, history, directory):
        curve = self.sampled(history["rolling"])
        environment = result["metadata"]["config"]["environment"]
        target = environment["target_distance_m"] / 1000
        tolerance = environment["distance_tolerance"]

        # Match plot_training_charts in the reference; only CSV column names differ.
        figure, axes = plt.subplots(3, 2, figsize=(14, 13))
        figure.suptitle(
            self.chart_title([result]),
            fontsize=14, fontweight="bold",
        )
        panels = [
            ("is_loop", f"Rata-rata Keberhasilan {self.rolling_window} Episode Terakhir", "Proporsi is_loop", "#16A34A"),
            ("total_reward", f"Rata-rata Reward {self.rolling_window} Episode Terakhir", "Reward", "#2563EB"),
            ("distance_km", f"Rata-rata Jarak {self.rolling_window} Episode Terakhir", "Jarak (km)", "#7C3AED"),
            ("mean_comfort", f"Rata-rata Skor Kenyamanan {self.rolling_window} Episode Terakhir", "Comfort score", "#F59E0B"),
            ("q_state_count", "Jumlah State Q-table", "Jumlah state", "#DC2626"),
            ("epsilon", "Nilai Epsilon per Episode", "Epsilon", "#0891B2"),
        ]
        for axis, (column, heading, unit, color) in zip(axes.flat, panels):
            values = self.sampled(history["means"])[column] if column == "epsilon" else curve[column]
            axis.plot(values.index, values, color=color)
            axis.set(title=heading, xlabel="Episode", ylabel=unit)
            axis.grid(alpha=0.25)
        axes[0, 0].set_ylim(-0.02, 1.02)
        axes[1, 0].axhline(target, color="black", linestyle="--", label=f"Target {target:g} km")
        axes[1, 0].axhline(target * (1 - tolerance), color="#9CA3AF", linestyle=":", label="Batas valid")
        axes[1, 0].axhline(target * (1 + tolerance), color="#9CA3AF", linestyle=":")
        axes[1, 0].legend()
        # The reference fixes 0..1; retain that view unless it would hide actual scores.
        scores = history["rolling"]["mean_comfort"].dropna()
        lower, upper = 0.0, 1.0
        if not scores.empty and (scores.min() < lower or scores.max() > upper):
            lower, upper = min(lower, scores.min()), max(upper, scores.max())
            padding = (upper - lower) * 0.05
            lower, upper = lower - padding, upper + padding
        axes[1, 1].set_ylim(lower, upper)
        axes[2, 1].set_ylim(0, 1.05)
        self.save_figure(figure, directory, "training")

        # Match plot_diagnosis_charts, with termination rates spanning both columns.
        figure = plt.figure(figsize=(14, 14))
        grid = figure.add_gridspec(3, 2)
        axes = [figure.add_subplot(grid[0, 0]), figure.add_subplot(grid[0, 1]),
                figure.add_subplot(grid[1, 0]), figure.add_subplot(grid[1, 1])]
        diagnosis = [
            ("distance_to_start_at_end_m", f"Rata-rata Jarak Akhir ke Start {self.rolling_window} Episode Terakhir", "Meter", 1, "#0F766E"),
            ("return_progress_ratio", "Konsistensi Bergerak Mendekati Start", "Proporsi langkah", 1, "#2563EB"),
            ("closing_action_available", "Peluang Closing Action Tersedia", "Rate (%)", 100, "#16A34A"),
            ("mean_abs_td_error", f"Rata-rata Nilai Absolut TD-error {self.rolling_window} Episode Terakhir", "TD-error", 1, "#9333EA"),
        ]
        for axis, (column, heading, unit, scale, color) in zip(axes, diagnosis):
            axis.plot(curve.index, curve[column] * scale, color=color)
            axis.set(title=heading, xlabel="Episode", ylabel=unit)
            axis.grid(alpha=0.25)
            if not curve[column].notna().any():
                axis.text(0.5, 0.5, "Belum ada observasi", transform=axis.transAxes, ha="center")
        axes[1].set_ylim(-0.02, 1.02)
        axes[2].set_ylim(-1, 101)
        axis = figure.add_subplot(grid[2, :])
        rates = self.sampled(history["termination_rolling"])
        labels = TERMINATION_LABELS.copy()
        labels[1] = f"Melebihi {target * (1 + tolerance):g} km".replace(".", ",")
        for reason, label, color in zip(TERMINATIONS, labels, TERMINATION_COLORS):
            axis.plot(rates.index, rates[reason] * 100, label=label, color=color, linewidth=2)
        axis.set(title=f"Termination Rate {self.rolling_window} Episode Terakhir",
                 xlabel="Episode", ylabel="Rate (%)", ylim=(-1, 101))
        axis.legend(title="Tipe terminasi", fontsize=8)
        axis.grid(alpha=0.25)
        figure.suptitle(
            self.chart_title([result]),
            fontsize=14, fontweight="bold",
        )
        self.save_figure(figure, directory, "diagnosis")

        # Match plot_overlap_chart: one panel, all episodes and all observed seeds.
        figure, axis = plt.subplots(figsize=(12, 4.5))
        figure.suptitle(
            self.chart_title([result]),
            fontsize=14, fontweight="bold",
        )
        overlap = self.sampled(history["overlap"])
        axis.plot(overlap.index, overlap * 100, color="#2563EB")
        if not overlap.notna().any():
            axis.text(0.5, 0.5, "Belum ada episode dengan data overlap",
                      transform=axis.transAxes, ha="center", va="center")
        axis.set(title="Seluruh Episode — Berdasarkan Jumlah Lintasan Ruas",
                 xlabel="Episode", ylabel="Lintasan ulang (%)", ylim=(-1, 101))
        axis.grid(alpha=0.25)
        self.save_figure(figure, directory, "overlap")

    def evaluation_summary(self, result):
        frame = result["evaluation"]
        training = result["metadata"]["config"]["training"]
        horizon = training["epsilon_decay_episodes"]
        return {
            "scenario": result["scenario"], "profile": result["name"], "episodes": training["episodes"],
            "decay_mode": "dynamic" if horizon is None else "fixed",
            "decay_horizon": training["episodes"] if horizon is None else horizon,
            "seed_count": len(frame), "loop_count": int(frame["is_loop"].sum()),
            "success_rate_greedy": frame["is_loop"].mean(),
            "mean_distance_error_m": frame["absolute_distance_error_m"].mean(),
            "mean_total_reward": frame["total_reward"].mean(), "mean_comfort": frame["mean_comfort"].mean(),
            "mean_overlap_ratio": frame["overlap_ratio"].mean(),
            "mean_return_progress_ratio": frame["return_progress_ratio"].mean(),
            "closing_action_available_rate": frame["closing_action_available"].mean(),
            "mean_distance_to_start_at_end_m": frame["distance_to_start_at_end_m"].mean(),
            "mean_q_state_count": frame["q_state_count"].mean(),
        }

    def comparison_signature(self, result):
        metadata = result["metadata"]
        training = metadata["config"]["training"]
        shared = {key: metadata.get(key) for key in [
            "snapshot_sha256", "start_node", "graph_simplified", "start_distance_method", "rng",
            "reward_scheme", "reward_constants", "road_score", "access_policy", "action_mask",
            "action_identity", "overlap_basis", "physical_road_identity", "evaluation_epsilon", "evaluation_seed_offset",
            "q_formula", "epsilon_formula", "tie_break",
        ]}
        shared["environment"] = metadata["config"]["environment"]
        shared["reward"] = metadata["config"].get("reward")
        shared["seeds"] = result["seeds"]
        shared["hyperparameters"] = {key: training[key] for key in ["alpha", "gamma", "epsilon_start", "epsilon_end"]}
        return json.dumps(shared, sort_keys=True)

    def plot_comparison(self, frame, directory, title, labels):
        # Supplementary matched A/B/C comparison, using the same four metric panels.
        figure, axes = plt.subplots(2, 2, figsize=(13, 8))
        panels = [
            ("success_rate_greedy", "Keberhasilan Loop pada Evaluasi Greedy", "Proporsi seed"),
            ("mean_distance_error_m", "Rata-rata Galat Jarak", "Meter"),
            ("mean_return_progress_ratio", "Konsistensi Bergerak Mendekati Start", "Proporsi langkah"),
            ("closing_action_available_rate", "Peluang Closing Action Tersedia", "Proporsi seed"),
        ]
        for axis, (column, heading, unit) in zip(axes.flat, panels):
            axis.bar(labels, frame[column], color="#2563EB")
            axis.set(title=heading, ylabel=unit)
            axis.grid(axis="y", alpha=0.25)
        figure.suptitle(title, fontsize=14, fontweight="bold")
        self.save_figure(figure, directory, "evaluation_comparison")
        figure, axis = plt.subplots(figsize=(10, 4.5))
        axis.bar(labels, frame["mean_overlap_ratio"] * 100, color="#2563EB")
        figure.suptitle(title, fontsize=14, fontweight="bold")
        axis.set(title="Overlap Seluruh Seed Evaluasi",
                 ylabel="Overlap seluruh seed (%)", ylim=(-1, 101))
        axis.grid(axis="y", alpha=0.25)
        self.save_figure(figure, directory, "evaluation_overlap")

    def plot_profile_sensitivity(self, frame, directory, title):
        # Match section 3.1/3.2 in scenario_A/B/C.ipynb: four line plots by episodes.
        figure, axes = plt.subplots(2, 2, figsize=(13, 8))
        panels = [
            ("success_rate_greedy", "Keberhasilan Loop pada Evaluasi Greedy", "Proporsi seed"),
            ("mean_distance_error_m", "Rata-rata Galat Jarak", "Meter"),
            ("mean_return_progress_ratio", "Konsistensi Bergerak Mendekati Start", "Proporsi langkah"),
            ("closing_action_available_rate", "Peluang Closing Action Tersedia", "Proporsi seed"),
        ]
        for axis, (column, heading, unit) in zip(axes.flat, panels):
            axis.plot(frame["episodes"], frame[column], marker="o")
            axis.set(title=heading, xlabel="Jumlah episode training", ylabel=unit)
            axis.grid(alpha=0.25)
        figure.suptitle(title, fontsize=14, fontweight="bold")
        self.save_figure(figure, directory, "evaluation_comparison")

    def load(self):
        """Read completed runs and reject missing, incomplete, or stale results."""
        self.results = []
        self.analysis_dir = None
        self.profile_summary = None
        self.last_window_summary = None
        plt.rcdefaults()
        pd.set_option("display.max_columns", 25)
        self.output_dir, self.results, availability = self.discover_results(self.workdir, self.profiles)
        display(pd.DataFrame(availability))
        if not self.results:
            raise FileNotFoundError("No completed training results. Run the Go train command first.")
        self.analysis_dir = self.output_dir / "analysis" / "training"
        self.analysis_dir.mkdir(parents=True, exist_ok=True)

    def show_configurations(self):
        """Display and export the settings recorded with the selected runs."""
        self._require_loaded()
        configuration_rows = []
        for result in self.results:
            metadata = result["metadata"]
            training = metadata["config"]["training"]
            environment = metadata["config"]["environment"]
            configuration_rows.append({
                "scenario": result["scenario"], "profile": result["name"],
                "episodes": training["episodes"], "epsilon_decay_episodes": training["epsilon_decay_episodes"],
                "alpha": training["alpha"], "gamma": training["gamma"],
                "epsilon_start": training["epsilon_start"], "epsilon_end": training["epsilon_end"],
                "seeds": ", ".join(result["seeds"]), "target_distance_m": environment["target_distance_m"],
                "tolerance": environment["distance_tolerance"], "max_steps": environment["max_steps"],
                "graph_simplified": metadata["graph_simplified"], "snapshot_sha256": metadata["snapshot_sha256"],
            })
        configurations = pd.DataFrame(configuration_rows)
        display(configurations)
        configurations.to_csv(self.analysis_dir / "configurations.csv", index=False)

    def show_profiles(self):
        """Display per-seed metrics and charts; export numerical summaries."""
        self._require_loaded()
        tail_rows = []
        for result in self.results:
            training = result["metadata"]["config"]["training"]
            horizon = training["epsilon_decay_episodes"]
            decay_label = "dinamis" if horizon is None else f"{horizon:,} episode"
            print(f"Scenario {result['scenario']} | {training['episodes']:,} episode | epsilon decay: {decay_label}")
            print(f"Profil: {result['name']}")
            print("=== METRIK EVALUASI GREEDY PER SEED (HASIL TERSIMPAN) ===")
            seed_order = sorted(result["seeds"], key=int)
            display(result["evaluation"].set_index("seed").loc[seed_order, EVALUATION_DISPLAY_COLUMNS].T)

            evaluation = result["evaluation"]
            seed_count = len(evaluation)
            if seed_count == 1:
                print("=== DIAGNOSIS EVALUASI: SATU TRAINING ===")
                print("Satu baris ini merangkum satu evaluasi greedy dari training tersebut.")
            else:
                print(f"=== RINGKASAN DIAGNOSIS EVALUASI: AGREGAT {seed_count} SEED ===")
                print(f"Satu baris ini adalah rata-rata/proporsi dari {seed_count} evaluasi greedy, "
                      "bukan record satu seed. Detail setiap seed ada pada tabel sebelumnya.")
            diagnosis = pd.DataFrame([{
                "scenario": result["scenario"],
                "distance_to_start_at_end_mean_m": evaluation["distance_to_start_at_end_m"].mean(),
                "return_progress_ratio_mean": evaluation["return_progress_ratio"].mean(),
                "closing_action_available_rate": evaluation["closing_action_available"].mean(),
                "q_state_count_mean": evaluation["q_state_count"].mean(),
            }])
            display(diagnosis)

            history = self.read_history(result)
            directory = self.analysis_dir / result["name"]
            directory.mkdir(parents=True, exist_ok=True)
            training_termination = self.termination_table(history["termination_counts"])
            evaluation_termination = self.termination_table(evaluation["termination_reason"].value_counts())
            print("=== TERMINATION RATE: TRAINING ===")
            display(training_termination)
            print("=== TERMINATION RATE: EVALUASI GREEDY ===")
            display(evaluation_termination)
            print("=== GRAFIK TRAINING, DIAGNOSIS, DAN OVERLAP RUTE ===")
            print("overlap_ratio adalah proporsi lintasan ulang, disimpan 0–1; grafik dalam persen.")
            self.plot_training(result, history, directory)

            # Keep additional numerical summaries without adding panels to the reference view.
            diagnosis.to_csv(directory / "diagnosis_summary.csv", index=False)
            history["means"].to_csv(directory / "episode_means.csv")
            history["overlap"].rename("overlap_ratio").to_csv(directory / "overlap_rolling.csv")
            history["tail_by_seed"].to_csv(directory / "last_window_by_seed.csv")
            training_termination.to_csv(directory / "termination_training.csv", index=False)
            evaluation_termination.to_csv(directory / "termination_evaluation.csv", index=False)
            tail_rows.append({"scenario": result["scenario"], "profile": result["name"],
                              "episodes_in_window": history["tail_episode_count"], **history["tail_mean"].to_dict()})
        self.last_window_summary = pd.DataFrame(tail_rows)
        self.last_window_summary.to_csv(self.analysis_dir / "last_window_summary.csv", index=False)

    def compare_profiles(self):
        """Compare episode counts within each recorded scenario separately."""
        self._require_loaded()
        summary = pd.DataFrame([self.evaluation_summary(result) for result in self.results])
        summary.to_csv(self.analysis_dir / "profile_summary.csv", index=False)
        base = self.analysis_dir
        try:
            for scenario in sorted({result["scenario"] for result in self.results}):
                results = [result for result in self.results if result["scenario"] == scenario]
                self.analysis_dir = base / f"scenario-{scenario.lower()}"
                self.analysis_dir.mkdir(parents=True, exist_ok=True)
                self._compare_profile_group(results)
        finally:
            self.analysis_dir = base
            self.profile_summary = summary

    def _compare_profile_group(self, results):
        """Compare episode counts separately for each epsilon strategy."""
        self._require_loaded()
        self.profile_summary = pd.DataFrame([self.evaluation_summary(result) for result in results])
        self.profile_summary.to_csv(self.analysis_dir / "profile_summary.csv", index=False)
        if len({result["scenario"] for result in results}) != 1 or len({self.comparison_signature(result) for result in results}) != 1:
            raise ValueError("Profiles have mismatched graph/reward/seeds/settings; review metadata before comparing.")

        summary_columns = [
            "episodes", "decay_horizon", "success_rate_greedy", "mean_distance_error_m",
            "mean_total_reward", "mean_comfort", "mean_return_progress_ratio",
            "closing_action_available_rate", "mean_overlap_ratio",
        ]
        strategy_groups = []
        dynamic = self.profile_summary.loc[self.profile_summary["decay_mode"].eq("dynamic")].sort_values("episodes")
        if not dynamic.empty:
            strategy_groups.append(("Epsilon dinamis", "dynamic", dynamic))
        fixed = self.profile_summary.loc[self.profile_summary["decay_mode"].eq("fixed")]
        for horizon, frame in fixed.groupby("decay_horizon", sort=True):
            label = f"Horizon epsilon {horizon / 1000:g}K"
            strategy_groups.append((label, f"horizon-{int(horizon)}", frame.sort_values("episodes")))

        for label, name, frame in strategy_groups:
            print(f"=== {label.upper()} ===")
            display(frame[summary_columns].rename(columns={"decay_horizon": "epsilon_decay_episodes"}).reset_index(drop=True))
            directory = self.analysis_dir / "comparison" / name
            selected = [result for result in results if result["name"] in set(frame["profile"])]
            title = self.chart_title(selected)
            self.plot_profile_sensitivity(frame, directory, title)
            frame.to_csv(directory / "summary.csv", index=False)

        # Match section 3.3: one overlap panel for each available epsilon strategy.
        figure, axes = plt.subplots(1, len(strategy_groups), figsize=(6.5 * len(strategy_groups), 4), squeeze=False)
        for axis, (label, _, frame) in zip(axes.flat, strategy_groups):
            values = frame["mean_overlap_ratio"] * 100
            axis.plot(frame["episodes"], values, marker="o", color="#2563EB")
            if not values.notna().any():
                axis.text(0.5, 0.5, "Belum ada data overlap", ha="center", va="center", transform=axis.transAxes)
            axis.set(title=label, xlabel="Jumlah episode training", ylabel="Overlap seluruh seed (%)", ylim=(-1, 101))
            axis.grid(alpha=0.25)
        figure.suptitle(self.chart_title(results),
                        fontsize=14, fontweight="bold")
        self.save_figure(figure, self.analysis_dir, "evaluation_overlap")
        # Remove the older combined overview after the replacement strategy charts are saved.
        for extension in ("png", "svg"):
            (self.analysis_dir / f"evaluation_comparison.{extension}").unlink(missing_ok=True)

    def compare_scenarios(self, workdirs=None):
        """Compare scenarios only when their recorded settings match."""
        self._require_loaded()
        if workdirs is None:
            comparison_records, comparison_availability = list(self.results), []
        else:
            comparison_records, comparison_availability = [], []
        workdirs = [] if workdirs is None else [resolve_workdir(workdir) for workdir in workdirs]
        for workdir in workdirs:
            if not (workdir / "config.yaml").is_file():
                comparison_availability.append({"workdir": workdir.name, "profile": "—", "status": "config belum tersedia"})
                continue
            _, records, available = self.discover_results(workdir)
            comparison_records.extend(records)
            comparison_availability.extend(available)
        display(pd.DataFrame(comparison_availability))
        if comparison_records:
            grouped_records = {}
            for result in comparison_records:
                row = self.evaluation_summary(result)
                key = (row["episodes"], row["decay_mode"], row["decay_horizon"])
                grouped_records.setdefault(key, []).append(result)
            cross_rows = []
            for key, group in grouped_records.items():
                matched = len({self.comparison_signature(result) for result in group}) == 1
                if len({result["scenario"] for result in group}) != len(group):
                    print(f"{key}: beberapa profil untuk scenario yang sama; pilih satu profil per scenario.")
                    continue
                state_count = len({result["scenario"] for result in group})
                status = "matched" if matched and state_count >= 2 else ("one scenario only" if state_count < 2 else "mismatched metadata")
                for result in group:
                    cross_rows.append({**self.evaluation_summary(result), "comparison_status": status})
                if state_count < 2:
                    print(f"{key[0]}: perbandingan state belum tersedia (minimal dua scenario).")
                    continue
                if not matched:
                    print(f"{key[0]}: metadata tidak sepadan; grafik perbandingan state tidak dibuat.")
                    continue
                frame = pd.DataFrame([self.evaluation_summary(result) for result in group]).sort_values("scenario")
                name = f"{key[1]}-{key[0]}-horizon-{key[2]}"
                title = self.chart_title(group)
                self.plot_comparison(frame, self.analysis_dir / "comparison" / name, title, frame["scenario"].tolist())
            cross_summary = pd.DataFrame(cross_rows)
            display(cross_summary.round(4))
            cross_summary.to_csv(self.analysis_dir / "state_comparison.csv", index=False)
