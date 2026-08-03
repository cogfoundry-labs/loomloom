#!/usr/bin/env bash
set -euo pipefail

REPOSITORY=""
TAG=""
BRANCH=""
SYNC_MAIN="false"

usage() {
  cat <<'EOF'
Usage: scripts/sync-gitee-release-refs.sh --repo <owner/repo> --tag <tag> [options]

Compatibility wrapper for scripts/sync-release-refs.sh.

Options:
  --repo <owner/repo>  Gitee repository
  --tag <tag>          Existing stable or prerelease tag
  --branch <branch>    Source branch to synchronize before the tag
  --sync-main <bool>   Legacy stable-release option (default: false)
  --help               Show this help text
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo)
      REPOSITORY="${2:-}"
      shift 2
      ;;
    --tag)
      TAG="${2:-}"
      shift 2
      ;;
    --branch)
      BRANCH="${2:-}"
      shift 2
      ;;
    --sync-main)
      SYNC_MAIN="${2:-}"
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

if [[ "$SYNC_MAIN" != "true" && "$SYNC_MAIN" != "false" ]]; then
  echo "--sync-main must be true or false" >&2
  exit 1
fi
if [[ -n "$BRANCH" && "$SYNC_MAIN" == "true" ]]; then
  echo "--branch and --sync-main true cannot be used together" >&2
  exit 1
fi
if [[ -z "$BRANCH" && "$SYNC_MAIN" == "true" ]]; then
  BRANCH="main"
fi
if [[ -z "$BRANCH" && ! "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "legacy tag-only synchronization supports stable release tags only" >&2
  exit 1
fi

args=(
  --provider gitee
  --repo "$REPOSITORY"
  --tag "$TAG"
)
if [[ -n "$BRANCH" ]]; then
  args+=(--branch "$BRANCH")
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "$script_dir/sync-release-refs.sh" "${args[@]}"
