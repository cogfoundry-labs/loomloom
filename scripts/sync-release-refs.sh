#!/usr/bin/env bash
set -euo pipefail

PROVIDER=""
REPOSITORY=""
TAG=""
BRANCH=""
ALLOW_HISTORY="false"
MAX_ATTEMPTS="${RELEASE_SYNC_MAX_ATTEMPTS:-5}"
REMOTE_REF_SHA=""
REMOTE_TAG_COMMIT_SHA=""
SYNC_REMOTE="release-sync"
PROVIDER_LABEL=""
HTTP_LOW_SPEED_LIMIT=1024
HTTP_LOW_SPEED_TIME=60

usage() {
  cat <<'EOF'
Usage: scripts/sync-release-refs.sh --provider <gitee|gitlab> --repo <owner/repo> --tag <tag> [options]

Synchronize an immutable release tag and, when provided, its source branch to a mirror repository.

Options:
  --provider <name>   Mirror provider: gitee or gitlab
  --repo <owner/repo> Mirror repository
  --tag <tag>         Existing stable or prerelease tag
  --branch <branch>   Source branch to synchronize before the tag
  --allow-history     Allow the tag to be an ancestor of the source branch HEAD
  --help              Show this help text

Environment:
  GITEE_SYNC_TOKEN          Gitee token with repository write access
  GITLAB_SYNC_TOKEN         GitLab token with repository write access
  RELEASE_SYNC_MAX_ATTEMPTS Retry limit (default: 5)
  RELEASE_SYNC_REMOTE_URL   Remote URL override used by local tests
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --provider)
      PROVIDER="${2:-}"
      shift 2
      ;;
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
    --allow-history)
      ALLOW_HISTORY="true"
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

case "$PROVIDER" in
  gitee)
    PROVIDER_LABEL="Gitee"
    sync_token="${GITEE_SYNC_TOKEN:-}"
    default_remote_url="https://oauth2:${sync_token}@gitee.com/${REPOSITORY}.git"
    ;;
  gitlab)
    PROVIDER_LABEL="GitLab"
    sync_token="${GITLAB_SYNC_TOKEN:-}"
    default_remote_url="https://oauth2:${sync_token}@gitlab.com/${REPOSITORY}.git"
    ;;
  *)
    echo "--provider must be gitee or gitlab" >&2
    exit 1
    ;;
esac

if [[ ! "$REPOSITORY" =~ ^[^/]+/[^/]+$ ]]; then
  echo "--repo must use owner/repo format" >&2
  exit 1
fi
if [[ ! "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-(internal|beta|rc)\.[0-9]+)?$ ]]; then
  echo "unsupported release tag for ${PROVIDER_LABEL} synchronization: $TAG" >&2
  exit 1
fi
if [[ -n "$BRANCH" ]]; then
  if [[ "$BRANCH" != "main" &&
        "$BRANCH" != "refactor/phase-i-repo-structure" &&
        ! "$BRANCH" =~ ^release/v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "unsupported release branch: $BRANCH" >&2
    exit 1
  fi
fi
if [[ "$ALLOW_HISTORY" == "true" && -z "$BRANCH" ]]; then
  echo "--allow-history requires --branch" >&2
  exit 1
fi
if [[ -z "$sync_token" ]]; then
  echo "${PROVIDER^^}_SYNC_TOKEN is required" >&2
  exit 1
fi
if [[ ! "$MAX_ATTEMPTS" =~ ^[1-9][0-9]*$ ]]; then
  echo "RELEASE_SYNC_MAX_ATTEMPTS must be a positive integer" >&2
  exit 1
fi

tag_ref="refs/tags/${TAG}"
release_sha="$(git rev-list -n 1 "$tag_ref")"
local_tag_object="$(git rev-parse "$tag_ref")"
remote_url="${RELEASE_SYNC_REMOTE_URL:-$default_remote_url}"
branch_target_sha="$release_sha"

if [[ -n "$BRANCH" ]]; then
  git fetch --no-tags origin \
    "refs/heads/${BRANCH}:refs/remotes/origin/${BRANCH}"
  source_branch_sha="$(git rev-parse "refs/remotes/origin/${BRANCH}")"
  branch_target_sha="$source_branch_sha"
  if [[ "$ALLOW_HISTORY" == "true" ]]; then
    if ! git merge-base --is-ancestor "$release_sha" "$source_branch_sha"; then
      echo "release tag does not belong to origin/${BRANCH}: $TAG" >&2
      exit 1
    fi
  elif [[ "$release_sha" != "$source_branch_sha" ]]; then
    echo "release tag does not point at origin/${BRANCH} HEAD: $TAG" >&2
    exit 1
  fi
fi

git remote remove "$SYNC_REMOTE" >/dev/null 2>&1 || true
git remote add "$SYNC_REMOTE" "$remote_url"
trap 'git remote remove "$SYNC_REMOTE" >/dev/null 2>&1 || true' EXIT

retry_delay() {
  local attempt="$1"
  sleep $((attempt * 2))
}

run_network_git() {
  GIT_TERMINAL_PROMPT=0 git \
    -c http.lowSpeedLimit="$HTTP_LOW_SPEED_LIMIT" \
    -c http.lowSpeedTime="$HTTP_LOW_SPEED_TIME" \
    "$@"
}

read_remote_ref() {
  local ref="$1"
  local attempt=1
  local output=""
  local command_exit=0

  while [[ "$attempt" -le "$MAX_ATTEMPTS" ]]; do
    echo "checking ${PROVIDER_LABEL} ref $ref ($attempt/$MAX_ATTEMPTS)" >&2
    if output="$(
      run_network_git ls-remote --refs "$SYNC_REMOTE" "$ref"
    )"; then
      REMOTE_REF_SHA="$(awk 'NR == 1 { print $1 }' <<< "$output")"
      return 0
    else
      command_exit=$?
    fi

    if [[ "$attempt" -ge "$MAX_ATTEMPTS" ]]; then
      echo "failed to query ${PROVIDER_LABEL} ref after $MAX_ATTEMPTS attempts: $ref" >&2
      return "$command_exit"
    fi
    echo "${PROVIDER_LABEL} ref query failed; retrying $ref ($attempt/$MAX_ATTEMPTS)" >&2
    retry_delay "$attempt"
    attempt=$((attempt + 1))
  done
}

read_remote_tag() {
  local attempt=1
  local output=""
  local command_exit=0
  local peeled_ref="${tag_ref}^{}"

  REMOTE_REF_SHA=""
  REMOTE_TAG_COMMIT_SHA=""

  while [[ "$attempt" -le "$MAX_ATTEMPTS" ]]; do
    echo "checking ${PROVIDER_LABEL} tag $TAG ($attempt/$MAX_ATTEMPTS)" >&2
    if output="$(
      run_network_git ls-remote "$SYNC_REMOTE" "$tag_ref" "$peeled_ref"
    )"; then
      REMOTE_REF_SHA="$(awk -v ref="$tag_ref" '$2 == ref { print $1; exit }' <<< "$output")"
      REMOTE_TAG_COMMIT_SHA="$(awk -v ref="$peeled_ref" '$2 == ref { print $1; exit }' <<< "$output")"
      if [[ -n "$REMOTE_REF_SHA" && -z "$REMOTE_TAG_COMMIT_SHA" ]]; then
        REMOTE_TAG_COMMIT_SHA="$REMOTE_REF_SHA"
      fi
      return 0
    else
      command_exit=$?
    fi

    if [[ "$attempt" -ge "$MAX_ATTEMPTS" ]]; then
      echo "failed to query ${PROVIDER_LABEL} tag after $MAX_ATTEMPTS attempts: $TAG" >&2
      return "$command_exit"
    fi
    echo "${PROVIDER_LABEL} tag query failed; retrying $TAG ($attempt/$MAX_ATTEMPTS)" >&2
    retry_delay "$attempt"
    attempt=$((attempt + 1))
  done
}

remote_tag_matches_source() {
  [[ "$REMOTE_REF_SHA" == "$local_tag_object" ]] || \
    [[ -n "$REMOTE_TAG_COMMIT_SHA" && "$REMOTE_TAG_COMMIT_SHA" == "$release_sha" ]]
}

report_remote_tag_match() {
  if [[ "$REMOTE_REF_SHA" == "$local_tag_object" ]]; then
    echo "${PROVIDER_LABEL} tag already matches the source repository: $TAG"
  else
    echo "${PROVIDER_LABEL} tag points to the same release commit with different tag metadata: $TAG"
  fi
}

push_ref() {
  run_network_git push --progress "$@"
}

preflight_tag() {
  read_remote_tag
  if remote_tag_matches_source; then
    report_remote_tag_match
    return
  fi
  if [[ -n "$REMOTE_REF_SHA" ]]; then
    echo "${PROVIDER_LABEL} tag differs from the source repository: $TAG" >&2
    echo "published tags are immutable; refusing to modify ${PROVIDER_LABEL} refs" >&2
    exit 1
  fi
}

sync_tag() {
  local attempt=1
  local push_exit=0

  while [[ "$attempt" -le "$MAX_ATTEMPTS" ]]; do
    read_remote_tag
    if remote_tag_matches_source; then
      report_remote_tag_match
      return
    fi
    if [[ -n "$REMOTE_REF_SHA" ]]; then
      echo "${PROVIDER_LABEL} tag differs from the source repository: $TAG" >&2
      echo "published tags are immutable; refusing to overwrite the ${PROVIDER_LABEL} tag" >&2
      exit 1
    fi

    push_exit=0
    echo "pushing ${PROVIDER_LABEL} tag $TAG ($attempt/$MAX_ATTEMPTS)" >&2
    if push_ref "$SYNC_REMOTE" "${tag_ref}:${tag_ref}"; then
      :
    else
      push_exit=$?
    fi

    # A failed push can still have reached the mirror before the response was lost.
    read_remote_tag
    if remote_tag_matches_source; then
      echo "confirmed ${PROVIDER_LABEL} tag: $TAG"
      return
    fi
    if [[ -n "$REMOTE_REF_SHA" ]]; then
      echo "${PROVIDER_LABEL} tag differs from the source repository after push: $TAG" >&2
      exit 1
    fi
    if [[ "$push_exit" -eq 0 ]]; then
      echo "${PROVIDER_LABEL} accepted the tag push but the tag is not visible; stopping safely" >&2
      exit 1
    fi
    if [[ "$attempt" -ge "$MAX_ATTEMPTS" ]]; then
      echo "failed to push ${PROVIDER_LABEL} tag after $MAX_ATTEMPTS attempts: $TAG" >&2
      exit 1
    fi

    echo "${PROVIDER_LABEL} tag push failed and the tag is still absent; retrying ($attempt/$MAX_ATTEMPTS)" >&2
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
  if [[ "$initial_branch" == "$branch_target_sha" ]]; then
    echo "${PROVIDER_LABEL} $branch already matches source branch: $branch_target_sha"
    return
  fi

  while [[ "$attempt" -le "$MAX_ATTEMPTS" ]]; do
    push_exit=0
    echo "pushing ${PROVIDER_LABEL} branch $branch ($attempt/$MAX_ATTEMPTS)" >&2
    if [[ -n "$initial_branch" ]]; then
      if push_ref \
        --force-with-lease="${branch_ref}:${initial_branch}" \
        "$SYNC_REMOTE" "${branch_target_sha}:${branch_ref}"; then
        :
      else
        push_exit=$?
      fi
    elif push_ref "$SYNC_REMOTE" "${branch_target_sha}:${branch_ref}"; then
      :
    else
      push_exit=$?
    fi

    # Refuse to overwrite a branch that changed after the initial read.
    read_remote_ref "$branch_ref"
    if [[ "$REMOTE_REF_SHA" == "$branch_target_sha" ]]; then
      echo "synced $TAG to ${PROVIDER_LABEL} $branch"
      return
    fi
    if [[ "$REMOTE_REF_SHA" != "$initial_branch" ]]; then
      echo "${PROVIDER_LABEL} $branch changed unexpectedly; refusing to overwrite it" >&2
      exit 1
    fi
    if [[ "$push_exit" -eq 0 ]]; then
      echo "${PROVIDER_LABEL} accepted the branch push but $branch is unchanged; stopping safely" >&2
      exit 1
    fi
    if [[ "$attempt" -ge "$MAX_ATTEMPTS" ]]; then
      echo "failed to sync ${PROVIDER_LABEL} $branch after $MAX_ATTEMPTS attempts" >&2
      exit 1
    fi

    echo "${PROVIDER_LABEL} $branch push failed and the ref is unchanged; retrying ($attempt/$MAX_ATTEMPTS)" >&2
    retry_delay "$attempt"
    attempt=$((attempt + 1))
  done
}

echo "starting ${PROVIDER_LABEL} release synchronization: branch=${BRANCH:-unchanged}, tag=$TAG" >&2
preflight_tag
if [[ -n "$BRANCH" ]]; then
  sync_branch "$BRANCH"
else
  echo "kept ${PROVIDER_LABEL} branches unchanged for $TAG"
fi
sync_tag
