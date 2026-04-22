#!/usr/bin/env python3
"""
gwt — Git worktree manager

Usage:
  gwt create <name>    Create worktree at <repo>/worktrees/w_<name> on branch b_<name>
  gwt list             List all worktrees for the current repo
  gwt switch [name]    Print path of worktree to switch to (shell wrapper does cd)
  gwt delete [name]    Delete a worktree (warns on uncommitted/unpushed changes)
  gwt help             Show this help

fzf is used for interactive selection when no name is given.
Falls back to a numbered list if fzf is not installed.
"""

import argparse
import subprocess
import sys
from pathlib import Path
from typing import Optional


def get_main_root() -> Path:
    """Return the main worktree root. Works even when called from inside a worktree."""
    result = subprocess.run(
        ["git", "worktree", "list", "--porcelain"],
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        sys.exit("error: not in a git repo")
    for line in result.stdout.splitlines():
        if line.startswith("worktree "):
            return Path(line.split(" ", 1)[1])
    sys.exit("error: not in a git repo")


def worktrees_dir(main: Path) -> Path:
    return main / "worktrees"


def pick(prompt: str, root: Path) -> Optional[str]:
    """Interactively pick a worktree name via fzf, falling back to a numbered list."""
    if not root.exists():
        sys.exit("error: no worktrees directory found — run 'gwt create' first")
    names = sorted(d.name for d in root.iterdir() if d.is_dir())
    if not names:
        sys.exit("error: no worktrees found")

    # Try fzf first
    fzf = subprocess.run(
        ["fzf", f"--prompt={prompt}", "--height=10"],
        input="\n".join(names),
        capture_output=True,
        text=True,
    )
    if fzf.returncode == 0:
        return fzf.stdout.strip()
    if fzf.returncode == 130:  # user pressed Esc/Ctrl-C in fzf
        return None

    # fzf not available — numbered list fallback
    for i, name in enumerate(names, 1):
        print(f"  {i}) {name}")
    try:
        raw = input(f"{prompt}(number): ").strip()
        idx = int(raw) - 1
        if 0 <= idx < len(names):
            return names[idx]
    except (ValueError, EOFError, KeyboardInterrupt):
        pass
    return None


# ── subcommands ──────────────────────────────────────────────────────────────

def cmd_create(args: argparse.Namespace) -> None:
    main = get_main_root()
    path = worktrees_dir(main) / f"w_{args.name}"
    subprocess.run(
        ["git", "worktree", "add", str(path), "-b", f"b_{args.name}"],
        check=True,
    )


def cmd_list(_args: argparse.Namespace) -> None:
    main = get_main_root()
    result = subprocess.run(
        ["git", "worktree", "list"], capture_output=True, text=True, check=True
    )
    print(f"Repo: {main.name}")
    for line in result.stdout.splitlines()[1:]:  # skip main worktree line
        parts = line.split()
        if len(parts) >= 3:
            name = Path(parts[0]).name
            print(f"  {name:<28} {parts[1]}  {parts[2]}")


def cmd_switch(args: argparse.Namespace) -> None:
    """Prints the target path — the shell wrapper is responsible for cd."""
    main = get_main_root()
    root = worktrees_dir(main)
    if args.name:
        target = root / f"w_{args.name}"
    else:
        chosen = pick("switch worktree> ", root)
        if not chosen:
            sys.exit(0)
        target = root / chosen
    print(target)


def cmd_delete(args: argparse.Namespace) -> None:
    main = get_main_root()
    root = worktrees_dir(main)
    if args.name:
        wt_name = f"w_{args.name}"
    else:
        chosen = pick("delete worktree> ", root)
        if not chosen:
            sys.exit(0)
        wt_name = chosen

    wt_path = root / wt_name
    warnings = []

    status = subprocess.run(
        ["git", "-C", str(wt_path), "status", "--porcelain"],
        capture_output=True,
        text=True,
    )
    if status.stdout.strip():
        warnings.append("  - uncommitted changes")

    unpushed = subprocess.run(
        ["git", "-C", str(wt_path), "log", "@{u}..", "--oneline"],
        capture_output=True,
        text=True,
    )
    if unpushed.stdout.strip():
        warnings.append("  - unpushed commits")

    if warnings:
        print(f"Warning: '{wt_name}' has:")
        for w in warnings:
            print(w)
        prompt = "Delete anyway? [y/N] "
    else:
        prompt = f"Delete worktree '{wt_name}'? [y/N] "

    try:
        confirm = input(prompt)
    except (EOFError, KeyboardInterrupt):
        print("\nAborted.")
        sys.exit(0)

    if confirm.strip().lower() == "y":
        subprocess.run(["git", "worktree", "remove", str(wt_path)], check=True)
    else:
        print("Aborted.")


# ── CLI wiring ────────────────────────────────────────────────────────────────

def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="gwt",
        description="Git worktree manager",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )
    sub = parser.add_subparsers(dest="command")

    p_create = sub.add_parser("create", help="Create a new worktree")
    p_create.add_argument("name", help="Name — creates w_<name> on branch b_<name>")

    sub.add_parser("list", help="List all worktrees for the current repo")

    p_switch = sub.add_parser("switch", help="Switch into a worktree")
    p_switch.add_argument("name", nargs="?", help="Name (fzf picker if omitted)")

    p_delete = sub.add_parser("delete", help="Delete a worktree")
    p_delete.add_argument("name", nargs="?", help="Name (fzf picker if omitted)")

    return parser


COMMANDS = {
    "create": cmd_create,
    "list": cmd_list,
    "switch": cmd_switch,
    "delete": cmd_delete,
}


def main() -> None:
    parser = build_parser()
    args = parser.parse_args()
    if args.command not in COMMANDS:
        parser.print_help()
        sys.exit(0)
    COMMANDS[args.command](args)


if __name__ == "__main__":
    main()
