"""Shared paths and environment checks for CSV analysis notebooks."""

from pathlib import Path
import sys

import yaml

PROJECT_ROOT = Path(__file__).resolve().parents[1]


def resolve_workdir(workdir: str | Path) -> Path:
    path = Path(workdir).expanduser()
    return (path if path.is_absolute() else PROJECT_ROOT / path).resolve()


def read_workdir(workdir: str | Path) -> tuple[dict, Path]:
    workdir = resolve_workdir(workdir)
    config_path = workdir / "config.yaml"
    config = yaml.safe_load(config_path.read_text())
    relative = Path(str(config.get("output_dir", "output")).strip())
    if relative.is_absolute() or ".." in relative.parts or str(relative) == ".":
        raise ValueError(f"Invalid output_dir in {config_path}")
    return config, workdir / relative


def positive_integer(name: str, value: int) -> None:
    if isinstance(value, bool) or not isinstance(value, int) or value <= 0:
        raise ValueError(f"{name} must be a positive integer")


def show_environment(workdir: str | Path) -> None:
    if Path(sys.prefix).resolve() != (PROJECT_ROOT / ".venv").resolve():
        raise RuntimeError("Select the Running Route Analysis (Python 3.13) kernel.")
    print("Python:", sys.version.split()[0])
    print("Environment:", sys.prefix)
    print("Workdir:", resolve_workdir(workdir))
