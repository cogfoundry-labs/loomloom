#!/usr/bin/env python3
"""Generate LoomLoom text-v1 JSONL rows for batch code review."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Iterable


DEFAULT_EXTENSIONS = (
    ".go",
    ".py",
    ".js",
    ".jsx",
    ".ts",
    ".tsx",
    ".java",
    ".kt",
    ".rb",
    ".php",
    ".rs",
    ".swift",
    ".c",
    ".cc",
    ".cpp",
    ".h",
    ".hpp",
    ".cs",
    ".ex",
    ".exs",
)

DEFAULT_EXCLUDES = (
    ".git",
    ".idea",
    ".vscode",
    "node_modules",
    "dist",
    "build",
    "coverage",
    "vendor",
    ".next",
    "_build",
    "deps",
    "log",
    "tmp",
)

DEFAULT_PROMPT = (
    "Review this code file with focus on: "
    "1. memory, handle, connection, or other resource leak risks; "
    "2. privacy, sensitive information, credential, or user data leak risks; "
    "3. clearly unsafe implementation patterns; "
    "4. maintainability issues or error-prone implementation choices. "
    "If you do not find any high-confidence issues, say so explicitly."
)

DEFAULT_WRITING_REQUIREMENTS = (
    "Write in English. Judge only from the current file content and do not invent cross-file facts. "
    "Use this structure: "
    "[File conclusion] one-sentence summary; "
    "[Findings] sorted by severity, each with issue type / severity / evidence / rationale / recommendation; "
    "[Context gaps] call out any missing context needed for confirmation."
)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Generate text-v1 JSONL rows for batch code review."
    )
    parser.add_argument("--repo", required=True, help="Repository root to scan")
    parser.add_argument("--output", required=True, help="Output JSONL path")
    parser.add_argument(
        "--extensions",
        default=",".join(DEFAULT_EXTENSIONS),
        help="Comma-separated file extensions to include",
    )
    parser.add_argument(
        "--exclude-dirs",
        default=",".join(DEFAULT_EXCLUDES),
        help="Comma-separated directory names to exclude",
    )
    parser.add_argument(
        "--max-files",
        type=int,
        default=20,
        help="Maximum number of files to include",
    )
    parser.add_argument(
        "--max-bytes-per-file",
        type=int,
        default=12000,
        help="Maximum bytes of file content to inline per task row",
    )
    parser.add_argument(
        "--sort",
        choices=("largest", "path"),
        default="largest",
        help="How to order candidate files before truncating to --max-files",
    )
    parser.add_argument(
        "--prompt",
        default=DEFAULT_PROMPT,
        help="Value for text_prompts",
    )
    parser.add_argument(
        "--writing-requirements",
        default=DEFAULT_WRITING_REQUIREMENTS,
        help="Value for writing_requirements",
    )
    return parser.parse_args()


def should_skip(path: Path, excluded_dirs: set[str]) -> bool:
    return any(part in excluded_dirs for part in path.parts)


def discover_files(repo: Path, extensions: tuple[str, ...], excluded_dirs: set[str]) -> Iterable[Path]:
    for path in repo.rglob("*"):
        if not path.is_file():
            continue
        if should_skip(path.relative_to(repo), excluded_dirs):
            continue
        if path.suffix.lower() not in extensions:
            continue
        yield path


def truncate_utf8(data: bytes, limit: int) -> tuple[str, bool]:
    if len(data) <= limit:
        return data.decode("utf-8", errors="replace"), False
    truncated = data[:limit].decode("utf-8", errors="replace")
    return truncated, True


def language_for(path: Path) -> str:
    suffix = path.suffix.lower()
    return {
        ".go": "Go",
        ".py": "Python",
        ".js": "JavaScript",
        ".jsx": "JavaScript React",
        ".ts": "TypeScript",
        ".tsx": "TypeScript React",
        ".java": "Java",
        ".kt": "Kotlin",
        ".rb": "Ruby",
        ".php": "PHP",
        ".rs": "Rust",
        ".swift": "Swift",
        ".c": "C",
        ".cc": "C++",
        ".cpp": "C++",
        ".h": "C/C++ Header",
        ".hpp": "C++ Header",
        ".cs": "C#",
        ".ex": "Elixir",
        ".exs": "Elixir",
    }.get(suffix, suffix.lstrip(".") or "unknown")


def build_reference_text(repo: Path, path: Path, max_bytes: int) -> tuple[str, bool]:
    raw = path.read_bytes()
    content, was_truncated = truncate_utf8(raw, max_bytes)
    rel = path.relative_to(repo)
    lines = [
        f"repo_path: {repo}",
        f"file_path: {rel}",
        f"language: {language_for(path)}",
        "",
        "code:",
        content,
    ]
    if was_truncated:
        lines.extend(
            [
                "",
                "[truncated]",
                f"Only the first {max_bytes} bytes are included for this PoC.",
            ]
        )
    return "\n".join(lines), was_truncated


def main() -> int:
    args = parse_args()
    repo = Path(args.repo).expanduser().resolve()
    output = Path(args.output).expanduser().resolve()

    if not repo.exists() or not repo.is_dir():
        raise SystemExit(f"--repo is not a directory: {repo}")
    if args.max_files <= 0:
        raise SystemExit("--max-files must be greater than 0")
    if args.max_bytes_per_file <= 0:
        raise SystemExit("--max-bytes-per-file must be greater than 0")

    extensions = tuple(
        ext.strip().lower()
        for ext in args.extensions.split(",")
        if ext.strip()
    )
    excluded_dirs = {
        item.strip()
        for item in args.exclude_dirs.split(",")
        if item.strip()
    }

    files = list(discover_files(repo, extensions, excluded_dirs))
    if args.sort == "largest":
        files.sort(key=lambda path: path.stat().st_size, reverse=True)
    else:
        files.sort(key=lambda path: str(path.relative_to(repo)))

    selected = files[: args.max_files]
    output.parent.mkdir(parents=True, exist_ok=True)

    with output.open("w", encoding="utf-8") as handle:
        for path in selected:
            reference_text, _ = build_reference_text(repo, path, args.max_bytes_per_file)
            row = {
                "text_prompts": f"{args.prompt}\nTarget file: {path.relative_to(repo)}",
                "writing_requirements": args.writing_requirements,
                "reference_text": reference_text,
            }
            handle.write(json.dumps(row, ensure_ascii=False) + "\n")

    summary = {
        "repo": str(repo),
        "output": str(output),
        "selected_files": len(selected),
        "candidate_files": len(files),
        "extensions": list(extensions),
        "max_bytes_per_file": args.max_bytes_per_file,
    }
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
