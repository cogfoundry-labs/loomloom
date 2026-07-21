#!/usr/bin/env bash
set -euo pipefail

REPOSITORY=""
TAG=""
SYNC_MAIN="false"
MAX_ATTEMPTS=5
REMOTE_REF_SHA=""

usage() {
  cat <<'EOF'
Usage: scripts/sync-gitee-release-refs.sh --repo <owner/repo> --tag <tag> [options]

Synchronize an immutable release tag and, when requested, the release commit
to Gitee main.

Options:
  --repo <owner/repo>  Gitee repository
  --tag <tag>          Existing release tag
  --sync-main <bool>   Whether to update Gitee main (default: false)
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
if [[ ! "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-(beta|rc|internal)\.[0-9]+)?$ ]]; then
  echo "unsupported release tag: $TAG" >&2
  exit 1
fi
if [[ "$SYNC_MAIN" != "true" && "$SYNC_MAIN" != "false" ]]; then
  echo "--sync-main must be true or false" >&2
  exit 1
fi
if [[ -z "${GITEE_SYNC_TOKEN:-}" ]]; then
  echo "GITEE_SYNC_TOKEN is required" >&2
  exit 1
fi

tag_ref="refs/tags/${TAG}"
release_sha="$(git rev-list -n 1 "$tag_ref")"
local_tag_object="$(git rev-parse "$tag_ref")"
gitee_url="https://oauth2:${GITEE_SYNC_TOKEN}@gitee.com/${REPOSITORY}.git"

git remote add gitee "$gitee_url"

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
        ls-remote --refs gitee "$ref"
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
      echo "Gitee tag already matches GitHub: $TAG"
      return
    fi
    if [[ -n "$REMOTE_REF_SHA" ]]; then
      echo "Gitee tag differs from GitHub: $TAG" >&2
      echo "published tags are immutable; refusing to overwrite the Gitee tag" >&2
      exit 1
    fi

    push_exit=0
    if push_ref gitee "${tag_ref}:${tag_ref}"; then
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
      echo "Gitee tag differs from GitHub after push: $TAG" >&2
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

sync_main() {
  local attempt=1
  local initial_main=""
  local push_exit=0

  read_remote_ref refs/heads/main
  initial_main="$REMOTE_REF_SHA"
  if [[ "$initial_main" == "$release_sha" ]]; then
    echo "Gitee main already matches release commit: $release_sha"
    return
  fi

  while [[ "$attempt" -le "$MAX_ATTEMPTS" ]]; do
    push_exit=0
    if [[ -n "$initial_main" ]]; then
      if push_ref \
        --force-with-lease="refs/heads/main:${initial_main}" \
        gitee "${release_sha}:refs/heads/main"; then
        :
      else
        push_exit=$?
      fi
    elif push_ref gitee "${release_sha}:refs/heads/main"; then
      :
    else
      push_exit=$?
    fi

    # Re-read main after every push result. A lost response is successful when
    # the remote now points at the intended commit.
    read_remote_ref refs/heads/main
    if [[ "$REMOTE_REF_SHA" == "$release_sha" ]]; then
      echo "synced stable $TAG to Gitee main"
      return
    fi
    if [[ "$REMOTE_REF_SHA" != "$initial_main" ]]; then
      echo "Gitee main changed unexpectedly; refusing to overwrite it" >&2
      exit 1
    fi
    if [[ "$push_exit" -eq 0 ]]; then
      echo "Gitee accepted the main push but main is unchanged; stopping safely" >&2
      exit 1
    fi
    if [[ "$attempt" -ge "$MAX_ATTEMPTS" ]]; then
      echo "failed to sync Gitee main after $MAX_ATTEMPTS attempts" >&2
      exit 1
    fi

    echo "Gitee main push failed and the ref is unchanged; retrying ($attempt/$MAX_ATTEMPTS)" >&2
    retry_delay "$attempt"
    attempt=$((attempt + 1))
  done
}

sync_tag
if [[ "$SYNC_MAIN" == "true" ]]; then
  sync_main
else
  echo "kept Gitee main unchanged for $TAG"
fi
