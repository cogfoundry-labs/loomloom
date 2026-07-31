#!/usr/bin/env bash
set -euo pipefail

REPOSITORY=""
TAG=""
BRANCH=""
SYNC_MAIN="false"
MAX_ATTEMPTS=5
REMOTE_REF_SHA=""
GITEE_REMOTE="gitee-release-sync"

usage() {
  cat <<'EOF'
Usage: scripts/sync-gitee-release-refs.sh --repo <owner/repo> --tag <tag> [options]

Synchronize an immutable release tag and its source branch to Gitee.

Options:
  --repo <owner/repo>  Gitee repository
  --tag <tag>          Existing stable or prerelease tag
  --branch <branch>    Source branch to synchronize before the tag
  --sync-main <bool>   Legacy stable-release option (default: false)
  --help               Show this help text

Environment:
  GITEE_SYNC_TOKEN     Token with repository write access
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

if [[ ! "$REPOSITORY" =~ ^[^/]+/[^/]+$ ]]; then
  echo "--repo must use owner/repo format" >&2
  exit 1
fi
if [[ "$SYNC_MAIN" != "true" && "$SYNC_MAIN" != "false" ]]; then
  echo "--sync-main must be true or false" >&2
  exit 1
fi
if [[ -n "$BRANCH" ]]; then
  case "$BRANCH" in
    main|release/cogfoundry-v0.2.0) ;;
    *)
      echo "unsupported release branch: $BRANCH" >&2
      exit 1
      ;;
  esac
  if [[ ! "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-(internal|beta|rc)\.[0-9]+)?$ ]]; then
    echo "unsupported release tag for Gitee synchronization: $TAG" >&2
    exit 1
  fi
else
  if [[ ! "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "legacy synchronization only supports stable release tags: $TAG" >&2
    exit 1
  fi
fi
if [[ -z "${GITEE_SYNC_TOKEN:-}" ]]; then
  echo "GITEE_SYNC_TOKEN is required" >&2
  exit 1
fi

tag_ref="refs/tags/${TAG}"
release_sha="$(git rev-list -n 1 "$tag_ref")"
local_tag_object="$(git rev-parse "$tag_ref")"
gitee_url="https://oauth2:${GITEE_SYNC_TOKEN}@gitee.com/${REPOSITORY}.git"

if [[ -n "$BRANCH" ]]; then
  git fetch --no-tags origin \
    "refs/heads/${BRANCH}:refs/remotes/origin/${BRANCH}"
  source_branch_sha="$(git rev-parse "refs/remotes/origin/${BRANCH}")"
  if [[ "$release_sha" != "$source_branch_sha" ]]; then
    echo "release tag does not point at origin/${BRANCH} HEAD: $TAG" >&2
    exit 1
  fi
fi

git remote remove "$GITEE_REMOTE" >/dev/null 2>&1 || true
git remote add "$GITEE_REMOTE" "$gitee_url"
trap 'git remote remove "$GITEE_REMOTE" >/dev/null 2>&1 || true' EXIT

retry_delay() {
  local attempt="$1"
  sleep $((attempt * 2))
}

read_remote_ref() {
  local ref="$1"
  local attempt=1
  local output=""
  local command_exit=0

  while [[ "$attempt" -le "$MAX_ATTEMPTS" ]]; do
    if output="$(
      git \
        -c http.lowSpeedLimit=1 \
        -c http.lowSpeedTime=60 \
        ls-remote --refs "$GITEE_REMOTE" "$ref"
    )"; then
      REMOTE_REF_SHA="$(awk 'NR == 1 { print $1 }' <<< "$output")"
      return 0
    else
      command_exit=$?
    fi

    if [[ "$attempt" -ge "$MAX_ATTEMPTS" ]]; then
      echo "failed to query Gitee ref after $MAX_ATTEMPTS attempts: $ref" >&2
      return "$command_exit"
    fi
    echo "Gitee ref query failed; retrying $ref ($attempt/$MAX_ATTEMPTS)" >&2
    retry_delay "$attempt"
    attempt=$((attempt + 1))
  done
}

push_ref() {
  git \
    -c http.lowSpeedLimit=1 \
    -c http.lowSpeedTime=60 \
    push "$@"
}

sync_tag() {
  local attempt=1
  local push_exit=0

  while [[ "$attempt" -le "$MAX_ATTEMPTS" ]]; do
    read_remote_ref "$tag_ref"
    if [[ "$REMOTE_REF_SHA" == "$local_tag_object" ]]; then
      echo "Gitee tag already matches the source repository: $TAG"
      return
    fi
    if [[ -n "$REMOTE_REF_SHA" ]]; then
      echo "Gitee tag differs from the source repository: $TAG" >&2
      echo "published tags are immutable; refusing to overwrite the Gitee tag" >&2
      exit 1
    fi

    push_exit=0
    if push_ref "$GITEE_REMOTE" "${tag_ref}:${tag_ref}"; then
      :
    else
      push_exit=$?
    fi

    # A failed push can still have reached Gitee before the response was lost.
    read_remote_ref "$tag_ref"
    if [[ "$REMOTE_REF_SHA" == "$local_tag_object" ]]; then
      echo "confirmed Gitee tag: $TAG"
      return
    fi
    if [[ -n "$REMOTE_REF_SHA" ]]; then
      echo "Gitee tag differs from the source repository after push: $TAG" >&2
      exit 1
    fi
    if [[ "$push_exit" -eq 0 ]]; then
      echo "Gitee accepted the tag push but the tag is not visible; stopping safely" >&2
      exit 1
    fi
    if [[ "$attempt" -ge "$MAX_ATTEMPTS" ]]; then
      echo "failed to push Gitee tag after $MAX_ATTEMPTS attempts: $TAG" >&2
      exit 1
    fi

    echo "Gitee tag push failed and the tag is still absent; retrying ($attempt/$MAX_ATTEMPTS)" >&2
    retry_delay "$attempt"
    attempt=$((attempt + 1))
  done
}

sync_branch() {
  local branch="$1"
  local branch_ref="refs/heads/${branch}"
  local attempt=1
  local initial_branch=""
  local push_exit=0

  read_remote_ref "$branch_ref"
  initial_branch="$REMOTE_REF_SHA"
  if [[ "$initial_branch" == "$release_sha" ]]; then
    echo "Gitee $branch already matches release commit: $release_sha"
    return
  fi

  while [[ "$attempt" -le "$MAX_ATTEMPTS" ]]; do
    push_exit=0
    if [[ -n "$initial_branch" ]]; then
      if push_ref \
        --force-with-lease="${branch_ref}:${initial_branch}" \
        "$GITEE_REMOTE" "${release_sha}:${branch_ref}"; then
        :
      else
        push_exit=$?
      fi
    elif push_ref "$GITEE_REMOTE" "${release_sha}:${branch_ref}"; then
      :
    else
      push_exit=$?
    fi

    # Re-read the branch after every push result. A lost response is successful
    # when the remote now points at the intended commit.
    read_remote_ref "$branch_ref"
    if [[ "$REMOTE_REF_SHA" == "$release_sha" ]]; then
      echo "synced $TAG to Gitee $branch"
      return
    fi
    if [[ "$REMOTE_REF_SHA" != "$initial_branch" ]]; then
      echo "Gitee $branch changed unexpectedly; refusing to overwrite it" >&2
      exit 1
    fi
    if [[ "$push_exit" -eq 0 ]]; then
      echo "Gitee accepted the branch push but $branch is unchanged; stopping safely" >&2
      exit 1
    fi
    if [[ "$attempt" -ge "$MAX_ATTEMPTS" ]]; then
      echo "failed to sync Gitee $branch after $MAX_ATTEMPTS attempts" >&2
      exit 1
    fi

    echo "Gitee $branch push failed and the ref is unchanged; retrying ($attempt/$MAX_ATTEMPTS)" >&2
    retry_delay "$attempt"
    attempt=$((attempt + 1))
  done
}

if [[ -n "$BRANCH" ]]; then
  sync_branch "$BRANCH"
elif [[ "$SYNC_MAIN" == "true" ]]; then
  # Update the mirrored branch before publishing the tag. Gitee Go reads its
  # pipeline definition from main, while the tag push triggers the release.
  sync_branch main
else
  echo "kept Gitee branches unchanged for $TAG"
fi
sync_tag
