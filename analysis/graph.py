"""Analyze graph CSVs exported by Go; graph construction stays in Go."""

from pathlib import Path
import math

import matplotlib.pyplot as plt
import pandas as pd
from IPython.display import display

from .common import PROJECT_ROOT, positive_integer, read_workdir, resolve_workdir, show_environment

ATTRIBUTES = ["highway", "surface", "width", "access"]
NODE_COLUMNS = ["node_id", "latitude", "longitude"]
EDGE_COLUMNS = [
    "from", "to", "key", "way_id", "length_m", "name",
    "highway", "surface", "width", "access",
]


class GraphAnalysis:
    """Keep graph data and reusable report sections outside the notebook."""

    def __init__(self, workdir: str | Path, *, top_n: int = 10,
                 sample_rows: int = 5, length_bins: int = 50):
        for name, value in (("top_n", top_n), ("sample_rows", sample_rows),
                            ("length_bins", length_bins)):
            positive_integer(name, value)
        self.workdir = resolve_workdir(workdir)
        self.top_n = top_n
        self.sample_rows = sample_rows
        self.length_bins = length_bins

    def load(self):
        """Read original node/edge CSV values for the selected workdir."""
        for name in ("length_figure", "completeness_figure", "attributes_figure"):
            if hasattr(self, name):
                plt.close(getattr(self, name))
                delattr(self, name)
        _, self.output_dir = read_workdir(self.workdir)
        self.analysis_dir = self.output_dir / "analysis"
        plt.rcdefaults()
        plt.rcParams.update({"figure.dpi": 110, "axes.titlesize": 12, "axes.labelsize": 10})
        print("CSV directory:", self.output_dir)
        for filename in ("nodes.csv", "edges.csv"):
            if not (self.output_dir / filename).is_file():
                raise FileNotFoundError(f"Missing {filename}; run the Go analyze command for this workdir first.")

        self.nodes = pd.read_csv(
            self.output_dir / "nodes.csv", usecols=NODE_COLUMNS,
            dtype={"node_id": "int64", "latitude": "float64", "longitude": "float64"},
            keep_default_na=False,
        )
        self.edges = pd.read_csv(
            self.output_dir / "edges.csv", usecols=EDGE_COLUMNS,
            dtype={
                "from": "int64", "to": "int64", "key": "int64", "way_id": "int64",
                "length_m": "float64", "name": "string",
                **{attribute: "string" for attribute in ATTRIBUTES},
            },
            keep_default_na=False,
        )
        print(f"Loaded {len(self.nodes):,} nodes and {len(self.edges):,} directed edges.")
        self.attribute_values = self.edges[ATTRIBUTES].apply(lambda column: column.str.strip())
        self.edge_count = len(self.edges)

    def check_csv(self):
        """Display integrity checks and stop if graph data is inconsistent."""
        finite_coordinates = self.nodes[["latitude", "longitude"]].map(math.isfinite).all(axis=1)
        coordinates_in_range = self.nodes["latitude"].between(-90, 90) & self.nodes["longitude"].between(-180, 180)
        finite_lengths = self.edges["length_m"].map(math.isfinite)
        self.checks = pd.DataFrame([
            {"check": "Empty node table", "violations": int(self.nodes.empty)},
            {"check": "Empty edge table", "violations": int(self.edges.empty)},
            {"check": "Duplicate node IDs", "violations": int(self.nodes["node_id"].duplicated().sum())},
            {"check": "Duplicate (from, to, key)", "violations": int(self.edges.duplicated(["from", "to", "key"]).sum())},
            {"check": "Missing source nodes", "violations": int((~self.edges["from"].isin(self.nodes["node_id"])).sum())},
            {"check": "Missing destination nodes", "violations": int((~self.edges["to"].isin(self.nodes["node_id"])).sum())},
            {"check": "Invalid coordinates", "violations": int((~finite_coordinates | ~coordinates_in_range).sum())},
            {"check": "Invalid segment lengths", "violations": int((~finite_lengths | self.edges["length_m"].lt(0)).sum())},
        ])
        display(self.checks)
        if self.checks["violations"].gt(0).any():
            raise ValueError("CSV consistency checks failed. Inspect the table before continuing.")

    def show_summary(self):
        print("Node sample")
        display(self.nodes.head(self.sample_rows))
        print("Directed edge sample")
        display(self.edges.head(self.sample_rows))

        unique_destinations = self.edges.groupby("from")["to"].nunique().reindex(self.nodes["node_id"], fill_value=0)
        pair_sizes = self.edges.groupby(["from", "to"]).size()
        self.summary = pd.DataFrame([
            {"metric": "Nodes", "value": len(self.nodes)},
            {"metric": "Directed edges", "value": len(self.edges)},
            {"metric": "Source OSM ways represented", "value": self.edges["way_id"].nunique()},
            {"metric": "Nodes without outgoing edges", "value": int(unique_destinations.eq(0).sum())},
            {"metric": "Nodes with one unique destination", "value": int(unique_destinations.eq(1).sum())},
            {"metric": "Directed node pairs with parallel edges", "value": int(pair_sizes.gt(1).sum())},
            {"metric": "Zero-length directed edges", "value": int(self.edges["length_m"].eq(0).sum())},
        ])
        display(self.summary)

    def show_segment_lengths(self):
        self.length_statistics = self.edges["length_m"].describe(percentiles=[0.25, 0.5, 0.75, 0.95, 0.99])
        self.length_statistics = self.length_statistics.rename_axis("statistic").reset_index(name="value")
        display(self.length_statistics)

        self.length_figure, ax = plt.subplots(figsize=(9, 4), layout="constrained")
        ax.hist(self.edges["length_m"], bins=self.length_bins, color="tab:blue", edgecolor="white")
        ax.set(title="Raw segment length distribution", xlabel="Segment length (m)", ylabel="Directed edges")
        ax.grid(axis="y", alpha=0.2)
        plt.show()

    def show_completeness(self):
        self.completeness = pd.DataFrame([
            {
                "attribute": attribute,
                "present_edges": int(self.attribute_values[attribute].ne("").sum()),
                "missing_edges": int(self.attribute_values[attribute].eq("").sum()),
            }
            for attribute in ATTRIBUTES
        ])
        self.completeness["present_pct"] = self.completeness["present_edges"] / self.edge_count * 100
        self.completeness["missing_pct"] = self.completeness["missing_edges"] / self.edge_count * 100
        display(self.completeness.round(2))

        self.completeness_figure, ax = plt.subplots(figsize=(8, 4), layout="constrained")
        bars = ax.bar(self.completeness["attribute"], self.completeness["present_pct"], label="Present", color="tab:blue")
        ax.bar(self.completeness["attribute"], self.completeness["missing_pct"], bottom=self.completeness["present_pct"], label="Missing", color="lightgray")
        ax.bar_label(bars, labels=[f"{value:.1f}%" for value in self.completeness["present_pct"]], padding=3)
        ax.set(title="OSM attribute completeness", ylabel="Share of all directed edges (%)", ylim=(0, 112))
        ax.legend(loc="upper right")
        plt.show()

    def show_distributions(self):
        self.distributions = {}
        for attribute in ATTRIBUTES:
            values = self.attribute_values[attribute].replace("", "(missing)")
            table = values.value_counts().rename_axis("value").reset_index(name="edge_count")
            table["percentage"] = table["edge_count"] / self.edge_count * 100
            self.distributions[attribute] = table
            print(attribute)
            display(table.head(self.top_n).round(2))

        self.attributes_figure, axes = plt.subplots(2, 2, figsize=(14, 10), layout="constrained")
        for ax, attribute in zip(axes.flat, ATTRIBUTES):
            table = self.distributions[attribute].head(self.top_n)
            bars = ax.barh(table["value"], table["edge_count"], color="tab:blue")
            ax.invert_yaxis()
            ax.bar_label(bars, labels=[f"{count:,} ({percentage:.1f}%)" for count, percentage in zip(table["edge_count"], table["percentage"])], padding=3)
            ax.set(title=f"{attribute}: top {self.top_n} values", xlabel="Directed edges")
            ax.margins(x=0.4)
        plt.show()

    def show_width_quality(self):
        self.width_raw = self.attribute_values["width"]
        width_numeric = pd.to_numeric(self.width_raw, errors="coerce")
        self.positive_numeric_width = width_numeric.fillna(0).map(math.isfinite) & width_numeric.gt(0).fillna(False)
        width_status = pd.Series("Unparsed or non-positive", index=self.edges.index, dtype="string")
        width_status.loc[self.width_raw.eq("")] = "Missing"
        width_status.loc[self.positive_numeric_width & width_numeric.lt(3).fillna(False)] = "Positive numeric <3 m"
        width_status.loc[self.positive_numeric_width & width_numeric.ge(3).fillna(False)] = "Positive numeric >=3 m"
        width_order = ["Missing", "Unparsed or non-positive", "Positive numeric <3 m", "Positive numeric >=3 m"]
        self.width_quality = width_status.value_counts().reindex(width_order, fill_value=0).rename_axis("category").reset_index(name="edge_count")
        self.width_quality["percentage"] = self.width_quality["edge_count"] / self.edge_count * 100
        display(self.width_quality.round(2))

        print("Examples of non-empty width values not parsed as a positive number")
        display(self.width_raw.loc[self.width_raw.ne("") & ~self.positive_numeric_width].value_counts().head(self.top_n).rename_axis("raw_width").reset_index(name="edge_count"))

    def show_availability(self):
        self.combinations = self.attribute_values.ne("").value_counts().rename("edge_count").reset_index()
        self.combinations["percentage"] = self.combinations["edge_count"] / self.edge_count * 100
        display(self.combinations.round(2))

    def save(self):
        """Export the completed report tables and figures without changing inputs."""
        required = ("summary", "length_statistics", "completeness", "distributions",
                    "width_quality", "combinations", "length_figure", "completeness_figure", "attributes_figure")
        if any(not hasattr(self, name) for name in required):
            raise RuntimeError("Run the graph report sections before saving; use Run All in the notebook.")
        self.analysis_dir.mkdir(parents=True, exist_ok=True)
        all_distributions = pd.concat(
            [table.assign(attribute=attribute) for attribute, table in self.distributions.items()],
            ignore_index=True,
        )[["attribute", "value", "edge_count", "percentage"]]
        tables = {
            "graph_summary.csv": self.summary,
            "segment_length_statistics.csv": self.length_statistics,
            "attribute_completeness.csv": self.completeness,
            "attribute_distributions.csv": all_distributions,
            "width_quality.csv": self.width_quality,
            "attribute_availability_combinations.csv": self.combinations,
        }
        for filename, table in tables.items():
            table.to_csv(self.analysis_dir / filename, index=False)

        figures = {
            "segment_lengths.png": self.length_figure,
            "attribute_completeness.png": self.completeness_figure,
            "attribute_distributions.png": self.attributes_figure,
        }
        for filename, figure in figures.items():
            figure.savefig(self.analysis_dir / filename, dpi=180, bbox_inches="tight")

        print("Analysis files:", self.analysis_dir)
        for filename in [*tables, *figures]:
            print("-", filename)
