#!/usr/bin/env bash
set -euo pipefail

VERSION="${VERSION:-v0.1.0-local}"
AGENT="codex"
INSTALL_DIR="${INSTALL_DIR:-}"
SKILL_DIR="${SKILL_DIR:-}"

usage() {
  cat <<'EOF'
Usage: scripts/install-local.sh [options]

Build and install LoomLoom from the current local checkout.

Options:
  --agent <codex|claude|openclaw>   Install the matching skill pack (default: codex)
  --install-dir <path>              Directory for loomloom binary (default: current loomloom dir or ~/.local/bin)
  --skill-dir <path>                Override the destination directory for SKILL.md
  --version <version>               Version injected into the local CLI (default: v0.1.0-local)
  --help                            Show this help text
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --agent)
      AGENT="${2:-codex}"
      shift 2
      ;;
    --install-dir)
      INSTALL_DIR="${2:-}"
      shift 2
      ;;
    --skill-dir)
      SKILL_DIR="${2:-}"
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
      echo "unsupported agent for automatic skill install: $AGENT" >&2
      exit 1
      ;;
  esac
}

require_cmd go
require_cmd install

install_dir="$(resolve_install_dir)"
skill_dir="$(resolve_skill_dir)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

echo "LoomLoom local installer"
echo "repo: $repo_root"
echo "version: $VERSION"
echo "agent: $AGENT"
echo "install dir: $install_dir"
echo "skill dir: $skill_dir"
echo

mkdir -p "$install_dir" "$skill_dir"

(
  cd "$repo_root/cli"
  GOWORK=off go build \
    -ldflags "-X github.com/Cogfoundry-ai/loomloom/cli/internal/version.Version=${VERSION}" \
    -o "$tmp_dir/loomloom" \
    ./cmd/loomloom
)

install -m 0755 "$tmp_dir/loomloom" "$install_dir/loomloom"
cp -R "$repo_root/skills/$AGENT/loomloom/." "$skill_dir/"

echo "installed:"
echo "  $install_dir/loomloom"
echo "  $skill_dir/SKILL.md"
echo
"$install_dir/loomloom" --version
