#!/usr/bin/env bash
set -euo pipefail

VERSION="${VERSION:-v0.1.0-local}"
INSTALL_DIR="${INSTALL_DIR:-}"
SKILL_DIR=""

usage() {
  cat <<'EOF'
Usage: scripts/install-local.sh [options]

Build and install LoomLoom from the current local checkout.

Options:
  --install-dir <path>              Directory for loomloom binary (default: current loomloom dir or ~/.local/bin)
  --skill-dir <path>                Complete destination directory for the LoomLoom Skill
  --version <version>               Version injected into the local CLI (default: v0.1.0-local)
  --help                            Show this help text
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --install-dir)
      INSTALL_DIR="${2:-}"
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
    --version)
      VERSION="${2:-$VERSION}"
      shift 2
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

validate_loomloom_skill_frontmatter() {
  local skill_file="$1" line state="start" name_count=0 value
  [[ -f "$skill_file" && ! -L "$skill_file" ]] || return 1
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

if [[ -z "$SKILL_DIR" ]]; then
  echo "--skill-dir is required" >&2
  echo "Provide the complete destination ending in /loomloom." >&2
  exit 1
fi

SKILL_DIR="${SKILL_DIR%/}"
if [[ "$(basename -- "$SKILL_DIR")" != "loomloom" ]]; then
  echo "--skill-dir must be the complete LoomLoom Skill directory ending in /loomloom" >&2
  exit 1
fi

existing_skill_file="$SKILL_DIR/SKILL.md"
if [[ -e "$existing_skill_file" || -L "$existing_skill_file" ]]; then
  if ! validate_loomloom_skill_frontmatter "$existing_skill_file"; then
    echo "refusing to overwrite an existing non-LoomLoom Skill: $SKILL_DIR" >&2
    exit 1
  fi
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

resolve_install_dir() {
  if [[ -n "$INSTALL_DIR" ]]; then
    printf '%s\n' "$INSTALL_DIR"
    return
  fi
  local existing
  existing="$(command -v loomloom || true)"
  if [[ -n "$existing" ]]; then
    dirname "$existing"
    return
  fi
  printf '%s\n' "$HOME/.local/bin"
}

require_cmd go
require_cmd install

install_dir="$(resolve_install_dir)"
skill_dir="$SKILL_DIR"
if [[ ! -f "$repo_root/skills/loomloom/SKILL.md" ]]; then
  echo "local checkout does not contain skills/loomloom/SKILL.md" >&2
  exit 1
fi
tmp_dir="$(mktemp -d)"
docs_script="$repo_root/scripts/template-spec-docs.sh"
references_script="$repo_root/scripts/skill-references.sh"

cleanup() {
  rm -rf "$tmp_dir"
  "$references_script" clean
  "$docs_script" clean
}

trap cleanup EXIT

"$docs_script" prepare-checked
"$references_script" prepare-checked

echo "LoomLoom local installer"
echo "repo: $repo_root"
echo "version: $VERSION"
echo "install dir: $install_dir"
echo "skill dir: $skill_dir"
echo

mkdir -p "$install_dir" "$skill_dir"

(
  cd "$repo_root/cli"
  GOWORK=off go build \
    -ldflags "-X github.com/cogfoundry-labs/loomloom/cli/internal/version.Version=${VERSION}" \
    -o "$tmp_dir/loomloom" \
    ./cmd/loomloom
)

install -m 0755 "$tmp_dir/loomloom" "$install_dir/loomloom"
cp -R "$repo_root/skills/loomloom/." "$skill_dir/"

if [[ ! -f "$skill_dir/SKILL.md" ]]; then
  echo "Skill installation verification failed" >&2
  exit 1
fi

"$install_dir/loomloom" template-spec docs spec >/dev/null
"$install_dir/loomloom" template-spec docs spec --lang zh-CN >/dev/null

echo "installed:"
echo "  $install_dir/loomloom"
echo "  $skill_dir/SKILL.md"
echo
"$install_dir/loomloom" --version
