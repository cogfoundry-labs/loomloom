#!/usr/bin/env bash
set -euo pipefail

AGENT="codex"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
SKILL_DIR="${SKILL_DIR:-}"
REMOVE_CLI=1
REMOVE_SKILL=1
REMOVE_CONFIG=1
CLI_ONLY_REQUESTED=0
SKILL_ONLY_REQUESTED=0
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
      CLI_ONLY_REQUESTED=1
      shift
      ;;
    --skill-only)
      SKILL_ONLY_REQUESTED=1
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

if [[ "$CLI_ONLY_REQUESTED" -eq 1 && "$SKILL_ONLY_REQUESTED" -eq 1 ]]; then
  echo "cli-only and skill-only cannot be used together" >&2
  exit 1
fi
if [[ "$CLI_ONLY_REQUESTED" -eq 1 ]]; then
  REMOVE_SKILL=0
  REMOVE_CONFIG=0
elif [[ "$SKILL_ONLY_REQUESTED" -eq 1 ]]; then
  REMOVE_CLI=0
  REMOVE_CONFIG=0
fi

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

canonicalize_existing_dir() {
  local path="$1"
  (cd -P -- "$path" 2>/dev/null && pwd -P)
}

fail_skill_validation() {
  echo "refusing to remove unsafe Skill directory: $1" >&2
  return 1
}

validate_skill_frontmatter() {
  local skill_file="$1" line state="start" name_count=0 value
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line%$'\r'}"
    case "$state" in
      start)
        [[ "$line" == "---" ]] || return 1
        state="frontmatter"
        ;;
      frontmatter)
        if [[ "$line" == "---" ]]; then
          [[ "$name_count" -eq 1 ]] || return 1
          return 0
        fi
        if [[ "$line" =~ ^[[:space:]]*name[[:space:]]*:[[:space:]]*(.*)$ ]]; then
          value="${BASH_REMATCH[1]}"
          value="${value#"${value%%[![:space:]]*}"}"
          value="${value%"${value##*[![:space:]]}"}"
          case "$value" in
            loomloom|'"loomloom"'|"'loomloom'") ;;
            *) return 1 ;;
          esac
          name_count=$((name_count + 1))
        fi
        ;;
    esac
  done <"$skill_file"
  return 1
}

skill_references_file() {
  local skill_file="$1" reference_name="$2"
  grep -Fq -- "](references/$reference_name)" "$skill_file" ||
    grep -Fq -- "](references/$reference_name#" "$skill_file"
}

is_dangerous_skill_dir() {
  local candidate="$1" dangerous
  if [[ "$(dirname -- "$candidate")" == "$candidate" ]]; then
    return 0
  fi
  for dangerous in \
    "$HOME" \
    "$HOME/.codex/skills" \
    "$HOME/.claude/skills" \
    "$HOME/.openclaw/workspace/skills"; do
    if [[ -d "$dangerous" && "$candidate" -ef "$dangerous" ]]; then
      return 0
    fi
  done
  return 1
}

validate_skill_dir() {
  local requested_dir="$1" link_check_dir canonical_dir skill_file references_dir entry name

  link_check_dir="$requested_dir"
  while [[ "$link_check_dir" != "/" ]]; do
    link_check_dir="${link_check_dir%/}"
    if [[ "$link_check_dir" == */. ]]; then
      link_check_dir="${link_check_dir%/.}"
      continue
    fi
    break
  done
  if [[ -L "$link_check_dir" ]]; then
    fail_skill_validation "target is a symbolic link: $requested_dir"
    return 1
  fi
  if [[ ! -d "$requested_dir" ]]; then
    fail_skill_validation "target is not a directory: $requested_dir"
    return 1
  fi
  if ! canonical_dir="$(canonicalize_existing_dir "$requested_dir")"; then
    fail_skill_validation "cannot resolve target: $requested_dir"
    return 1
  fi
  if is_dangerous_skill_dir "$canonical_dir"; then
    fail_skill_validation "dangerous path: $canonical_dir"
    return 1
  fi

  skill_file="$canonical_dir/SKILL.md"
  references_dir="$canonical_dir/references"
  if [[ ! -f "$skill_file" || -L "$skill_file" ]]; then
    fail_skill_validation "SKILL.md is missing, is not a regular file, or is a symbolic link"
    return 1
  fi
  if ! validate_skill_frontmatter "$skill_file"; then
    fail_skill_validation "SKILL.md frontmatter must contain exactly 'name: loomloom'"
    return 1
  fi
  if [[ ! -d "$references_dir" || -L "$references_dir" ]]; then
    fail_skill_validation "references is missing, is not a directory, or is a symbolic link"
    return 1
  fi

  shopt -s dotglob nullglob
  for entry in "$canonical_dir"/*; do
    name="${entry##*/}"
    case "$name" in
      SKILL.md|references) ;;
      *)
        fail_skill_validation "unexpected top-level entry: $name"
        return 1
        ;;
    esac
  done

  for entry in "$references_dir"/*; do
    name="${entry##*/}"
    if [[ ! -f "$entry" || -L "$entry" ]]; then
      fail_skill_validation "unexpected reference entry: $name"
      return 1
    fi
    if ! skill_references_file "$skill_file" "$name"; then
      fail_skill_validation "reference is not explicitly referenced by SKILL.md: references/$name"
      return 1
    fi
  done

  printf '%s\n' "$canonical_dir"
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
final_skill_dir=""
skill_dir_present=0

if [[ "$REMOVE_SKILL" -eq 1 ]]; then
  requested_skill_dir="$(resolve_skill_dir)"
  if [[ -e "$requested_skill_dir" || -L "$requested_skill_dir" ]]; then
    final_skill_dir="$(validate_skill_dir "$requested_skill_dir")"
    skill_dir_present=1
  else
    final_skill_dir="$requested_skill_dir"
  fi
fi

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
  if [[ "$skill_dir_present" -eq 1 ]]; then
    rm -rf -- "$final_skill_dir"
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
