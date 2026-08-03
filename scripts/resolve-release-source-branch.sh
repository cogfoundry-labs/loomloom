#!/usr/bin/env bash
set -euo pipefail

TAG=""
ALLOW_HISTORY="false"
PRIMARY_REMOTE="${GIT_PRIMARY_REMOTE:-origin}"
RELEASE_BRANCH="${LOOMLOOM_RELEASE_BRANCH:-release/cogfoundry-v0.2.0}"

usage() {
  cat <<'EOF'
Usage: scripts/resolve-release-source-branch.sh <tag> [--allow-history]

Resolve the permitted source branch for a release tag.

Options:
  --allow-history  Accept a tag contained in an allowed branch instead of requiring branch HEAD
  --help           Show this help text
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --allow-history)
      ALLOW_HISTORY="true"
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    -*)
      echo "unknown argument: $1" >&2
      exit 1
      ;;
    *)
      if [[ -n "$TAG" ]]; then
        echo "release tag was provided more than once" >&2
        exit 1
      fi
      TAG="$1"
      shift
      ;;
  esac
done

if [[ ! "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-(internal|beta|rc)\.[0-9]+)?$ ]]; then
  echo "unsupported release tag: $TAG" >&2
  exit 1
fi

git fetch --no-tags "$PRIMARY_REMOTE" \
  "refs/heads/main:refs/remotes/${PRIMARY_REMOTE}/main"

tag_sha="$(git rev-list -n 1 "refs/tags/${TAG}")"
main_sha="$(git rev-parse "refs/remotes/${PRIMARY_REMOTE}/main")"

if [[ "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  if [[ "$tag_sha" == "$main_sha" ]]; then
    printf '%s\n' main
    exit 0
  fi
  if [[ "$ALLOW_HISTORY" == "true" ]] && \
    git merge-base --is-ancestor "$tag_sha" "$main_sha"; then
    printf '%s\n' main
    exit 0
  fi
  echo "stable release tag must point at ${PRIMARY_REMOTE}/main HEAD: $TAG" >&2
  exit 1
fi

git fetch --no-tags "$PRIMARY_REMOTE" \
  "refs/heads/${RELEASE_BRANCH}:refs/remotes/${PRIMARY_REMOTE}/${RELEASE_BRANCH}"
release_sha="$(git rev-parse "refs/remotes/${PRIMARY_REMOTE}/${RELEASE_BRANCH}")"

if [[ "$ALLOW_HISTORY" == "true" ]]; then
  # Prefer main when a historical prerelease commit is now contained in both
  # branches, because main is the canonical mirror branch.
  if git merge-base --is-ancestor "$tag_sha" "$main_sha"; then
    printf '%s\n' main
    exit 0
  fi
  if git merge-base --is-ancestor "$tag_sha" "$release_sha"; then
    printf '%s\n' "$RELEASE_BRANCH"
    exit 0
  fi

  echo "prerelease tag must belong to ${PRIMARY_REMOTE}/main or ${PRIMARY_REMOTE}/${RELEASE_BRANCH}: $TAG" >&2
  exit 1
fi

if [[ "$tag_sha" == "$release_sha" ]]; then
  printf '%s\n' "$RELEASE_BRANCH"
  exit 0
fi
if [[ "$tag_sha" == "$main_sha" ]]; then
  printf '%s\n' main
  exit 0
fi

echo "prerelease tag must point at ${PRIMARY_REMOTE}/main or ${PRIMARY_REMOTE}/${RELEASE_BRANCH} HEAD: $TAG" >&2
exit 1
