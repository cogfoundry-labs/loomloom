#!/usr/bin/env bash
set -euo pipefail

GITHUB_REPO="Cogfoundry-ai/loomloom"
GITEE_REPO="${GITEE_REPO:-cogfoundry/loomloom}"
VERSION="${VERSION:-latest}"
CHANNEL="${CHANNEL:-stable}"
AGENT="codex"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
SKILL_DIR="${SKILL_DIR:-}"
USE_HOMEBREW="auto"
HOMEBREW_TAP="${LOOMLOOM_HOMEBREW_TAP:-Cogfoundry-ai/tap}"
RELEASE_SOURCE="${LOOMLOOM_RELEASE_SOURCE:-github}"

usage() {
  cat <<'EOF'
Usage: install.sh [options]

Options:
  --agent <codex|claude|openclaw>   Install the matching skill pack (default: codex)
  --install-dir <path>     Directory for loomloom (default: ~/.local/bin)
  --skill-dir <path>       Override the destination directory for SKILL.md
  --version <tag|latest>   Release tag to install (default: latest)
  --channel <stable|beta|rc|internal>
                            Release channel to resolve when --version is latest (default: stable)
  --source <github|gitee>  Release source for version lookup and downloads (default: github)
  --no-brew                Force GitHub Release install even if Homebrew is available
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
    --version)
      VERSION="${2:-latest}"
      shift 2
      ;;
    --channel)
      CHANNEL="${2:-stable}"
      shift 2
      ;;
    --source)
      RELEASE_SOURCE="${2:-github}"
      shift 2
      ;;
    --no-brew)
      USE_HOMEBREW="never"
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

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  arm64|aarch64) ARCH="arm64" ;;
  x86_64|amd64) ARCH="amd64" ;;
  *)
    echo "unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

case "$CHANNEL" in
  stable|beta|rc|internal) ;;
  *)
    echo "unsupported release channel: $CHANNEL" >&2
    exit 1
    ;;
esac

case "$RELEASE_SOURCE" in
  github|gitee) ;;
  *)
    echo "unsupported release source: $RELEASE_SOURCE" >&2
    exit 1
    ;;
esac

mkdir -p "$INSTALL_DIR"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
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

resolve_tag() {
  if [[ "$VERSION" != "latest" ]]; then
    printf '%s\n' "$VERSION"
    return
  fi
  if [[ "$CHANNEL" != "stable" ]]; then
    resolve_prerelease_tag "$CHANNEL"
    return
  fi
  local tag
  tag="$(
    release_list_tags \
      | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' \
      | sort -V \
      | tail -n1
  )"
  if [[ -z "$tag" ]]; then
    echo "failed to resolve latest stable release tag" >&2
    exit 1
  fi
  printf '%s\n' "$tag"
}

release_list_tags() {
  # Prefer the git protocol (not subject to REST API rate limits and consistent across mirrors).
  if command -v git >/dev/null 2>&1; then
    local tags
    tags="$(git ls-remote --tags --refs "$(release_git_url)" 2>/dev/null \
      | sed -n 's#.*refs/tags/##p')"
    if [[ -n "$tags" ]]; then
      printf '%s\n' "$tags"
      return
    fi
  fi
  # Fallback to the REST API if git is unavailable.
  curl -fsSL "$(release_api_list_url)" \
    | tr '{' '\n' \
    | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p'
}

resolve_prerelease_tag() {
  local channel="$1"
  local tag
  tag="$(
    release_list_tags \
      | grep -E "^v[0-9]+\\.[0-9]+\\.[0-9]+-${channel}\\.[0-9]+$" \
      | sort -V \
      | tail -n1
  )"
  if [[ -z "$tag" ]]; then
    echo "failed to resolve latest $channel release tag" >&2
    exit 1
  fi
  printf '%s\n' "$tag"
}

can_use_homebrew() {
  [[ "$USE_HOMEBREW" != "never" ]] || return 1
  [[ "$RELEASE_SOURCE" == "github" ]] || return 1
  [[ "$VERSION" == "latest" ]] || return 1
  [[ "$CHANNEL" == "stable" ]] || return 1
  [[ -n "$HOMEBREW_TAP" ]] || return 1
  case "$OS" in
    darwin|linux) ;;
    *) return 1 ;;
  esac
  command -v brew >/dev/null 2>&1 || return 1
}

release_repo() {
  case "$RELEASE_SOURCE" in
    github) printf '%s\n' "$GITHUB_REPO" ;;
    gitee) printf '%s\n' "$GITEE_REPO" ;;
  esac
}

release_git_url() {
  case "$RELEASE_SOURCE" in
    github) printf 'https://github.com/%s.git\n' "$GITHUB_REPO" ;;
    gitee) printf 'https://gitee.com/%s.git\n' "$GITEE_REPO" ;;
  esac
}

release_api_list_url() {
  case "$RELEASE_SOURCE" in
    github) printf 'https://api.github.com/repos/%s/releases?per_page=100\n' "$GITHUB_REPO" ;;
    gitee) printf 'https://gitee.com/api/v5/repos/%s/releases?per_page=100\n' "$GITEE_REPO" ;;
  esac
}

release_download_base_url() {
  local tag="$1"
  case "$RELEASE_SOURCE" in
    github) printf 'https://github.com/%s/releases/download/%s\n' "$GITHUB_REPO" "$tag" ;;
    gitee) printf 'https://gitee.com/%s/releases/download/%s\n' "$GITEE_REPO" "$tag" ;;
  esac
}

checksum_tool() {
  if command -v sha256sum >/dev/null 2>&1; then
    printf '%s\n' "sha256sum"
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    printf '%s\n' "shasum -a 256"
    return
  fi
  printf '%s\n' ""
}

verify_checksum() {
  local tool="$1"
  local checksums_file="$2"
  local asset_name="$3"
  local asset_path="$4"
  local expected
  expected="$(awk -v name="$asset_name" '$2 == name { print $1 }' "$checksums_file")"
  if [[ -z "$expected" || -z "$tool" ]]; then
    return
  fi
  local actual
  actual="$($tool "$asset_path" | awk '{print $1}')"
  if [[ "$expected" != "$actual" ]]; then
    echo "checksum mismatch for $asset_name" >&2
    exit 1
  fi
}

require_cmd curl
SKILL_ASSET="loomloom-skills.tar.gz"
CHECKSUM_ASSET="checksums.txt"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

if can_use_homebrew; then
  TAG="homebrew-latest"
else
  require_cmd tar
  TAG="$(resolve_tag)"
  CLI_ASSET="loomloom-${OS}-${ARCH}.tar.gz"
  BASE_URL="$(release_download_base_url "$TAG")"
fi

echo "LoomLoom installer"
echo "repo: $(release_repo)"
echo "source: $RELEASE_SOURCE"
echo "version: $TAG"
echo "channel: $CHANNEL"
echo "agent: $AGENT"
if can_use_homebrew; then
  echo "cli install: homebrew"
else
  echo "install dir: $INSTALL_DIR"
fi
echo "skill dir: $(resolve_skill_dir)"
echo

if can_use_homebrew; then
  if brew list --versions loomloom >/dev/null 2>&1; then
    brew upgrade loomloom || true
  else
    brew install "$HOMEBREW_TAP/loomloom"
  fi
  local_cli_path="$INSTALL_DIR/loomloom"
  if [[ -f "$local_cli_path" ]]; then
    rm -f "$local_cli_path"
    echo "removed shadowing local CLI: $local_cli_path"
  fi
  CLI_PATH="$(command -v loomloom || true)"
  if [[ -z "$CLI_PATH" ]]; then
    echo "failed to resolve loomloom after Homebrew install" >&2
    exit 1
  fi
else
  curl -fsSL -o "$TMP_DIR/$CLI_ASSET" "$BASE_URL/$CLI_ASSET"
  curl -fsSL -o "$TMP_DIR/$CHECKSUM_ASSET" "$BASE_URL/$CHECKSUM_ASSET"
  VERIFY_TOOL="$(checksum_tool)"
  verify_checksum "$VERIFY_TOOL" "$TMP_DIR/$CHECKSUM_ASSET" "$CLI_ASSET" "$TMP_DIR/$CLI_ASSET"

  mkdir -p "$TMP_DIR/cli"
  tar -xzf "$TMP_DIR/$CLI_ASSET" -C "$TMP_DIR/cli"
  install -m 0755 "$TMP_DIR/cli/loomloom" "$INSTALL_DIR/loomloom"
  CLI_PATH="$INSTALL_DIR/loomloom"
fi

[[ -n "${BASE_URL:-}" ]] || BASE_URL="$(release_download_base_url "$(resolve_tag)")"
curl -fsSL -o "$TMP_DIR/$SKILL_ASSET" "$BASE_URL/$SKILL_ASSET"
if [[ ! -f "$TMP_DIR/$CHECKSUM_ASSET" ]]; then
  curl -fsSL -o "$TMP_DIR/$CHECKSUM_ASSET" "$BASE_URL/$CHECKSUM_ASSET"
fi
VERIFY_TOOL="${VERIFY_TOOL:-$(checksum_tool)}"
verify_checksum "$VERIFY_TOOL" "$TMP_DIR/$CHECKSUM_ASSET" "$SKILL_ASSET" "$TMP_DIR/$SKILL_ASSET"

mkdir -p "$TMP_DIR/skills"
tar -xzf "$TMP_DIR/$SKILL_ASSET" -C "$TMP_DIR/skills"
FINAL_SKILL_DIR="$(resolve_skill_dir)"
mkdir -p "$FINAL_SKILL_DIR"
cp -R "$TMP_DIR/skills/skills/$AGENT/loomloom/." "$FINAL_SKILL_DIR/"

echo "installed:"
echo "  $CLI_PATH"
echo "  $(resolve_skill_dir)/SKILL.md"
echo
echo "next:"
echo "  export LOOMLOOM_SERVER=<your LoomLoom server URL>"
echo "  export LOOMLOOM_TOKEN=your-token"
echo "  loomloom doctor"
