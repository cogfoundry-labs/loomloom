#!/usr/bin/env bash
set -euo pipefail

AGENT="codex"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
SKILL_DIR="${SKILL_DIR:-}"
REMOVE_CLI=1
REMOVE_SKILL=1
REMOVE_CONFIG=1
TOKEN_ENV_NAMES=""

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
      REMOVE_CONFIG=0
      shift
      ;;
    --skill-only)
      REMOVE_CLI=0
      REMOVE_SKILL=1
      REMOVE_CONFIG=0
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

resolve_config_file() {
  case "$(uname -s)" in
    Darwin)
      printf '%s\n' "$HOME/Library/Application Support/loomloom/config.json"
      ;;
    *)
      printf '%s\n' "${XDG_CONFIG_HOME:-$HOME/.config}/loomloom/config.json"
      ;;
  esac
}

add_token_env_name() {
  local name="$1"
  if [[ ! "$name" =~ ^LOOMLOOM_TOKEN(_[A-Z0-9_]+)?$ ]]; then
    return
  fi
  if [[ $'\n'"$TOKEN_ENV_NAMES"$'\n' == *$'\n'"$name"$'\n'* ]]; then
    return
  fi
  if [[ -z "$TOKEN_ENV_NAMES" ]]; then
    TOKEN_ENV_NAMES="$name"
  else
    TOKEN_ENV_NAMES="$TOKEN_ENV_NAMES"$'\n'"$name"
  fi
}

collect_config_token_env_names() {
  local config_file="$1" name
  if [[ ! -f "$config_file" ]]; then
    return
  fi
  while IFS= read -r name; do
    add_token_env_name "$name"
  done < <(
    sed -n 's/.*"token_env"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$config_file"
  )
}

collect_current_token_env_names() {
  local name
  while IFS= read -r name; do
    add_token_env_name "$name"
  done < <(compgen -e)
}

report_token_env_names() {
  local name
  if [[ -z "$TOKEN_ENV_NAMES" ]]; then
    return
  fi
  while IFS= read -r name; do
    echo "environment token cleanup required: $name"
  done <<<"$TOKEN_ENV_NAMES"
  echo "Agent action required: ask the user for confirmation before removing these variables from their permanent environment configuration."
}

removed_any=0
config_file=""

if [[ "$REMOVE_CONFIG" -eq 1 ]]; then
  config_file="$(resolve_config_file)"
  collect_config_token_env_names "$config_file"
  collect_current_token_env_names
fi

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
  if [[ -e "$final_skill_dir" || -L "$final_skill_dir" ]]; then
    rm -rf "$final_skill_dir"
    echo "removed: $final_skill_dir"
    removed_any=1
  else
    echo "not found: $final_skill_dir"
  fi
fi

if [[ "$REMOVE_CONFIG" -eq 1 ]]; then
  if [[ -f "$config_file" ]]; then
    rm -f "$config_file"
    rmdir "$(dirname "$config_file")" 2>/dev/null || true
    echo "removed: $config_file"
    removed_any=1
  else
    echo "not found: $config_file"
  fi
fi

if [[ "$removed_any" -eq 0 ]]; then
  echo "nothing removed"
fi

report_token_env_names
