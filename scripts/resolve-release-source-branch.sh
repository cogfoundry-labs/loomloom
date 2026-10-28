#!/usr/bin/env bash
set -euo pipefail

TAG=""
ALLOW_HISTORY="false"
PRIMARY_REMOTE="${GIT_PRIMARY_REMOTE:-origin}"
RELEASE_BRANCH="${LOOMLOOM_RELEASE_BRANCH:-}"

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

# Release model (see RELEASING.md):
# - vX.Y.0-beta.N and vX.Y.0 are cut from main.
# - vX.Y.Z-beta.N and vX.Y.Z with Z>0 are cut from release/X.Y, which receives only cherry-picked fixes.
# - A stable vX.Y.Z must point at the same commit as its highest vX.Y.Z-beta.N.
if [[ ! "$TAG" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)(-beta\.([0-9]+))?$ ]]; then
  echo "unsupported release tag: $TAG (expected vX.Y.Z or vX.Y.Z-beta.N)" >&2
  exit 1
fi
major="${BASH_REMATCH[1]}"
minor="${BASH_REMATCH[2]}"
patch="${BASH_REMATCH[3]}"
version="v${major}.${minor}.${patch}"

if [[ -z "$RELEASE_BRANCH" ]]; then
  if [[ "$patch" == "0" ]]; then
    RELEASE_BRANCH="main"
  else
    RELEASE_BRANCH="release/${major}.${minor}"
  fi
fi

tag_sha="$(git rev-list -n 1 "refs/tags/${TAG}")"

if [[ "$TAG" == "$version" ]]; then
  latest_beta="$(git tag --list "${version}-beta.*" | sort -t. -k4,4n | tail -n 1)"
  if [[ -z "$latest_beta" ]]; then
    echo "stable release $TAG needs a published ${version}-beta.N first" >&2
    exit 1
  fi
  if [[ "$(git rev-list -n 1 "refs/tags/${latest_beta}")" != "$tag_sha" ]]; then
    echo "stable release $TAG must point at the same commit as ${latest_beta}" >&2
    exit 1
  fi
fi

git fetch --no-tags "$PRIMARY_REMOTE" \
  "refs/heads/${RELEASE_BRANCH}:refs/remotes/${PRIMARY_REMOTE}/${RELEASE_BRANCH}"
branch_sha="$(git rev-parse "refs/remotes/${PRIMARY_REMOTE}/${RELEASE_BRANCH}")"

if [[ "$tag_sha" == "$branch_sha" ]]; then
  printf '%s\n' "$RELEASE_BRANCH"
  exit 0
fi
if [[ "$ALLOW_HISTORY" == "true" ]] && \
  git merge-base --is-ancestor "$tag_sha" "$branch_sha"; then
  printf '%s\n' "$RELEASE_BRANCH"
  exit 0
fi
# A stable release reuses its last beta commit, so the branch may have moved on since that beta.
if [[ "$TAG" == "$version" ]] && git merge-base --is-ancestor "$tag_sha" "$branch_sha"; then
  printf '%s\n' "$RELEASE_BRANCH"
  exit 0
fi

echo "release tag $TAG must point at ${PRIMARY_REMOTE}/${RELEASE_BRANCH} HEAD" >&2
exit 1
