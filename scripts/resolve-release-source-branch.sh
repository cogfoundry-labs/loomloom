#!/usr/bin/env bash
set -euo pipefail

TAG="${1:-}"
PRIMARY_REMOTE="${GIT_PRIMARY_REMOTE:-origin}"
RELEASE_BRANCH="${LOOMLOOM_RELEASE_BRANCH:-release/cogfoundry-v0.2.0}"

if [[ ! "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-(internal|beta|rc)\.[0-9]+)?$ ]]; then
  echo "unsupported release tag: $TAG" >&2
  exit 1
fi

git fetch --no-tags "$PRIMARY_REMOTE" \
  "refs/heads/main:refs/remotes/${PRIMARY_REMOTE}/main"

tag_sha="$(git rev-list -n 1 "refs/tags/${TAG}")"
main_sha="$(git rev-parse "refs/remotes/${PRIMARY_REMOTE}/main")"

if [[ "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  if [[ "$tag_sha" != "$main_sha" ]]; then
    echo "stable release tag must point at ${PRIMARY_REMOTE}/main HEAD: $TAG" >&2
    exit 1
  fi
  printf '%s\n' main
  exit 0
fi

git fetch --no-tags "$PRIMARY_REMOTE" \
  "refs/heads/${RELEASE_BRANCH}:refs/remotes/${PRIMARY_REMOTE}/${RELEASE_BRANCH}"
release_sha="$(git rev-parse "refs/remotes/${PRIMARY_REMOTE}/${RELEASE_BRANCH}")"

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
