#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
SKILL_DIR=""
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
  --install-dir <path>     Directory containing loomloom (default: ~/.local/bin)
  --skill-dir <path>       Complete LoomLoom Skill directory to remove
  --cli-only               Remove only the CLI
  --skill-only             Remove only the skill pack
  --help                   Show this help text
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --install-dir)
      INSTALL_DIR="${2:-$HOME/.local/bin}"
      shift 2
      ;;
    --skill-dir)
      if [[ $# -lt 2 || -z "${2:-}" || "${2:-}" == --* ]]; then
        echo "--skill-dir requires a value" >&2
        exit 1
      fi
      SKILL_DIR="$2"
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

if [[ "$REMOVE_SKILL" -eq 1 ]]; then
  if [[ -z "$SKILL_DIR" ]]; then
    echo "--skill-dir is required unless --cli-only is used" >&2
    exit 1
  fi
  if [[ "$SKILL_DIR" != "/" ]]; then
    SKILL_DIR="${SKILL_DIR%/}"
  fi
fi

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

is_dangerous_skill_dir() {
  local candidate="$1"
  if [[ "$(dirname -- "$candidate")" == "$candidate" ]]; then
    return 0
  fi
  if [[ -d "$HOME" && "$candidate" -ef "$HOME" ]]; then
    return 0
  fi
  return 1
}

validate_skill_dir() {
  local requested_dir="$1" link_check_dir canonical_dir skill_file references_dir generated_template_spec_dir

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
  if [[ "$(basename -- "$canonical_dir")" != "loomloom" ]]; then
    fail_skill_validation "target basename must be loomloom"
    return 1
  fi

  skill_file="$canonical_dir/SKILL.md"
  references_dir="$canonical_dir/references"
  generated_template_spec_dir="$canonical_dir/generated-template-spec"
  if [[ ! -e "$skill_file" && ! -L "$skill_file" ]]; then
    if [[ -e "$references_dir" || -L "$references_dir" ||
          ( -d "$generated_template_spec_dir" && ! -L "$generated_template_spec_dir" ) ]]; then
      fail_skill_validation "SKILL.md is missing while LoomLoom Skill directories are still present"
      return 1
    fi
    printf '%s\n' "$canonical_dir"
    return
  fi
  if [[ ! -f "$skill_file" || -L "$skill_file" ]]; then
    fail_skill_validation "SKILL.md is not a regular file or is a symbolic link"
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

  printf '%s\n' "$canonical_dir"
}

USER_SKILL_ENTRIES=()

collect_user_skill_entries() {
  local skill_dir="$1" entry name scan_file
  USER_SKILL_ENTRIES=()

  if ! scan_file="$(mktemp "${TMPDIR:-/tmp}/loomloom-uninstall.XXXXXX")"; then
    fail_skill_validation "cannot create temporary file for Skill directory scan"
    return 1
  fi
  if ! find "$skill_dir" -mindepth 1 -maxdepth 1 -print0 >"$scan_file"; then
    rm -f -- "$scan_file"
    fail_skill_validation "cannot enumerate Skill directory: $skill_dir"
    return 1
  fi
  while IFS= read -r -d '' entry; do
    name="${entry##*/}"
    case "$name" in
      SKILL.md|references)
        ;;
      generated-template-spec)
        if [[ ! -d "$entry" || -L "$entry" ]]; then
          USER_SKILL_ENTRIES+=("$entry")
        fi
        ;;
      *)
        USER_SKILL_ENTRIES+=("$entry")
        ;;
    esac
  done <"$scan_file"
  rm -f -- "$scan_file"
}

print_user_skill_entries() {
  local skill_dir="$1" entry relative
  for entry in "${USER_SKILL_ENTRIES[@]}"; do
    relative="${entry#"$skill_dir"/}"
    if [[ -d "$entry" && ! -L "$entry" ]]; then
      printf '  %q/\n' "$relative"
    else
      printf '  %q\n' "$relative"
    fi
  done
}

should_keep_user_skill_entries() {
  local answer use_tty=0
  echo "Detected user files in the LoomLoom Skill directory:"
  print_user_skill_entries "$final_skill_dir"

  if { : </dev/tty; } 2>/dev/null && { : >/dev/tty; } 2>/dev/null; then
    use_tty=1
  elif [[ -t 0 ]]; then
    use_tty=0
  else
    echo "non-interactive environment: preserving detected user files by default"
    return 0
  fi

  while true; do
    if [[ "$use_tty" -eq 1 ]]; then
      printf 'Keep these files? [Y/n] ' >/dev/tty
      if ! IFS= read -r answer </dev/tty 2>/dev/null; then
        echo "unable to read an interactive response: preserving detected user files by default"
        return 0
      fi
    else
      printf 'Keep these files? [Y/n] ' >&2
      if ! IFS= read -r answer; then
        echo "unable to read an interactive response: preserving detected user files by default"
        return 0
      fi
    fi
    answer="${answer#"${answer%%[![:space:]]*}"}"
    answer="${answer%"${answer##*[![:space:]]}"}"
    case "$answer" in
      ""|[yY]|[yY][eE][sS])
        return 0
        ;;
      [nN]|[nN][oO])
        return 1
        ;;
      *)
        if [[ "$use_tty" -eq 1 ]]; then
          echo "Please answer yes or no." >/dev/tty
        else
          echo "Please answer yes or no." >&2
        fi
        ;;
    esac
  done
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
skill_content_present=0
remove_entire_skill_dir=0
remove_user_skill_entries=0

if [[ "$REMOVE_SKILL" -eq 1 ]]; then
  requested_skill_dir="$SKILL_DIR"
  if [[ -e "$requested_skill_dir" || -L "$requested_skill_dir" ]]; then
    final_skill_dir="$(validate_skill_dir "$requested_skill_dir")"
    skill_dir_present=1
    collect_user_skill_entries "$final_skill_dir"
    if [[ -f "$final_skill_dir/SKILL.md" && ! -L "$final_skill_dir/SKILL.md" ]]; then
      skill_content_present=1
      if [[ "${#USER_SKILL_ENTRIES[@]}" -eq 0 ]]; then
        remove_entire_skill_dir=1
      elif ! should_keep_user_skill_entries; then
        remove_entire_skill_dir=1
        remove_user_skill_entries=1
      fi
    fi
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
    if [[ "$skill_content_present" -eq 0 ]]; then
      echo "no LoomLoom Skill content found: $final_skill_dir"
      if [[ "${#USER_SKILL_ENTRIES[@]}" -gt 0 ]]; then
        echo "preserved existing files in: $final_skill_dir"
        print_user_skill_entries "$final_skill_dir"
      fi
    elif [[ "$remove_entire_skill_dir" -eq 1 ]]; then
      rm -rf -- "$final_skill_dir"
      if [[ "$remove_user_skill_entries" -eq 1 ]]; then
        echo "removed Skill directory and detected user files: $final_skill_dir"
      else
        echo "removed: $final_skill_dir"
      fi
      removed_any=1
    else
      generated_template_spec_dir="$final_skill_dir/generated-template-spec"
      if [[ -d "$generated_template_spec_dir" && ! -L "$generated_template_spec_dir" ]]; then
        rm -rf -- "$generated_template_spec_dir"
      fi
      rm -rf -- "$final_skill_dir/references"
      rm -f -- "$final_skill_dir/SKILL.md"
      echo "removed LoomLoom Skill content from: $final_skill_dir"
      echo "preserved existing files in: $final_skill_dir"
      print_user_skill_entries "$final_skill_dir"
      removed_any=1
    fi
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
