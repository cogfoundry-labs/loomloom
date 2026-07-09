#!/usr/bin/env bash
set -euo pipefail

AGENT="codex"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
SKILL_DIR="${SKILL_DIR:-}"
REMOVE_CLI=1
REMOVE_SKILL=1

usage() {
  cat <<'EOF'
Usage: uninstall.sh [options]

Options:
  --agent <codex|claude|openclaw>   Remove the matching skill pack (default: codex)
  --install-dir <path>     Directory containing loomloom (default: ~/.local/bin)
  --skill-dir <path>       Override the destination directory for SKILL.md
  --cli-only               Remove only the CLI
  --skill-only             Remove only the skill pack
  --help                   Show this help text
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --agent)
      AGENT="${2:-codex}"
      shift 2
      ;;
    --install-dir)
      INSTALL_DIR="${2:-$HOME/.local/bin}"
      shift 2
      ;;
    --skill-dir)
      SKILL_DIR="${2:-}"
      shift 2
      ;;
    --cli-only)
      REMOVE_CLI=1
      REMOVE_SKILL=0
      shift
      ;;
    --skill-only)
      REMOVE_CLI=0
      REMOVE_SKILL=1
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

resolve_skill_dir() {
  if [[ -n "$SKILL_DIR" ]]; then
    printf '%s\n' "$SKILL_DIR"
    return
  fi
  case "$AGENT" in
    codex)
      printf '%s\n' "$HOME/.codex/skills/loomloom"
      ;;
    claude)
      printf '%s\n' "$HOME/.claude/skills/loomloom"
      ;;
    openclaw)
      printf '%s\n' "$HOME/.openclaw/workspace/skills/loomloom"
      ;;
    *)
      echo "unsupported agent for automatic skill uninstall: $AGENT" >&2
      exit 1
      ;;
  esac
}

removed_any=0

uninstall_homebrew_cli() {
  if ! command -v brew >/dev/null 2>&1; then
    return
  fi
  if ! brew list --versions loomloom >/dev/null 2>&1; then
    return
  fi
  brew uninstall loomloom
  echo "removed Homebrew formula: loomloom"
  removed_any=1
}

if [[ "$REMOVE_CLI" -eq 1 ]]; then
  uninstall_homebrew_cli
  cli_path="$INSTALL_DIR/loomloom"
  if [[ -f "$cli_path" ]]; then
    rm -f "$cli_path"
    echo "removed: $cli_path"
    removed_any=1
  else
    echo "not found: $cli_path"
  fi
fi

if [[ "$REMOVE_SKILL" -eq 1 ]]; then
  final_skill_dir="$(resolve_skill_dir)"
  skill_path="$final_skill_dir/SKILL.md"
  if [[ -f "$skill_path" ]]; then
    rm -f "$skill_path"
    rmdir "$final_skill_dir" 2>/dev/null || true
    echo "removed: $skill_path"
    removed_any=1
  else
    echo "not found: $skill_path"
  fi
fi

if [[ "$removed_any" -eq 0 ]]; then
  echo "nothing removed"
fi
