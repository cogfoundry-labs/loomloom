#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
syncer="$repo_root/scripts/sync-release-refs.sh"
test_root="$(mktemp -d)"
trap 'rm -rf "$test_root"' EXIT

fail_test() {
  echo "test failed: $*" >&2
  exit 1
}

assert_contains() {
  local output="$1"
  local expected="$2"
  [[ "$output" == *"$expected"* ]] || \
    fail_test "expected output to contain '$expected'"
}

assert_ref() {
  local repository="$1"
  local ref="$2"
  local expected="$3"
  local actual
  actual="$(git --git-dir="$repository" rev-parse "$ref")"
  [[ "$actual" == "$expected" ]] || \
    fail_test "$ref: expected '$expected', got '$actual'"
}

run_sync() {
  local provider="$1"
  local target="$2"
  local tag="$3"
  local branch="$4"
  local allow_history="${5:-false}"
  local token_name
  local args

  case "$provider" in
    gitee) token_name="GITEE_SYNC_TOKEN" ;;
    gitlab) token_name="GITLAB_SYNC_TOKEN" ;;
    *) fail_test "unsupported test provider: $provider" ;;
  esac

  args=(
    --provider "$provider"
    --repo cogfoundry/loomloom
    --branch "$branch"
    --tag "$tag"
  )
  if [[ "$allow_history" == "true" ]]; then
    args+=(--allow-history)
  fi

  (
    cd "$test_root/work"
    RELEASE_SYNC_REMOTE_URL="$target" \
      RELEASE_SYNC_MAX_ATTEMPTS=1 \
      env "$token_name=test-secret-token" \
      "$syncer" "${args[@]}"
  )
}

git init --bare "$test_root/origin.git" >/dev/null
git init -b main "$test_root/work" >/dev/null
git -C "$test_root/work" config user.name test
git -C "$test_root/work" config user.email test@example.com
git -C "$test_root/work" remote add origin "$test_root/origin.git"

printf '%s\n' first > "$test_root/work/state"
git -C "$test_root/work" add state
git -C "$test_root/work" commit -m first >/dev/null
first_sha="$(git -C "$test_root/work" rev-parse HEAD)"

printf '%s\n' second > "$test_root/work/state"
git -C "$test_root/work" commit -am second >/dev/null
release_sha="$(git -C "$test_root/work" rev-parse HEAD)"
git -C "$test_root/work" tag v1.0.0
git -C "$test_root/work" push origin main v1.0.0 >/dev/null

git init --bare "$test_root/gitee.git" >/dev/null
sync_output="$(run_sync gitee "$test_root/gitee.git" v1.0.0 main 2>&1)"
assert_contains "$sync_output" "starting Gitee release synchronization: branch=main, tag=v1.0.0"
assert_contains "$sync_output" "checking Gitee tag v1.0.0 (1/1)"
assert_contains "$sync_output" "pushing Gitee branch main (1/1)"
assert_contains "$sync_output" "pushing Gitee tag v1.0.0 (1/1)"
assert_ref "$test_root/gitee.git" refs/heads/main "$release_sha"
assert_ref "$test_root/gitee.git" refs/tags/v1.0.0 "$release_sha"

# An identical retry is successful and leaves both refs unchanged.
run_sync gitee "$test_root/gitee.git" v1.0.0 main >/dev/null
assert_ref "$test_root/gitee.git" refs/heads/main "$release_sha"
assert_ref "$test_root/gitee.git" refs/tags/v1.0.0 "$release_sha"

# GitLab uses the same synchronization rules through its own provider config.
git init --bare "$test_root/gitlab.git" >/dev/null
run_sync gitlab "$test_root/gitlab.git" v1.0.0 main >/dev/null
assert_ref "$test_root/gitlab.git" refs/heads/main "$release_sha"
assert_ref "$test_root/gitlab.git" refs/tags/v1.0.0 "$release_sha"

# A conflicting published tag is rejected before the branch is modified.
git --git-dir="$test_root/gitee.git" update-ref refs/heads/main "$first_sha"
git --git-dir="$test_root/gitee.git" update-ref refs/tags/v1.0.0 "$first_sha"
if run_sync gitee "$test_root/gitee.git" v1.0.0 main >/dev/null 2>&1; then
  fail_test "a different remote tag was overwritten"
fi
assert_ref "$test_root/gitee.git" refs/tags/v1.0.0 "$first_sha"
assert_ref "$test_root/gitee.git" refs/heads/main "$first_sha"

# A tag that is not at the selected source branch HEAD is rejected.
git -C "$test_root/work" tag v1.0.1 "$first_sha"
if run_sync gitlab "$test_root/gitlab.git" v1.0.1 main >/dev/null 2>&1; then
  fail_test "a tag outside the source branch HEAD was accepted"
fi

# Historical repair advances the mirror branch to the current source HEAD
# while preserving the historical tag commit.
printf '%s\n' third > "$test_root/work/state"
git -C "$test_root/work" commit -am third >/dev/null
current_main_sha="$(git -C "$test_root/work" rev-parse HEAD)"
git -C "$test_root/work" push origin main >/dev/null
git init --bare "$test_root/history.git" >/dev/null
run_sync gitlab "$test_root/history.git" v1.0.0 main true >/dev/null
assert_ref "$test_root/history.git" refs/heads/main "$current_main_sha"
assert_ref "$test_root/history.git" refs/tags/v1.0.0 "$release_sha"

# A concurrent branch update is preserved and causes synchronization to fail.
git -C "$test_root/work" switch -c concurrent-test >/dev/null
printf '%s\n' concurrent > "$test_root/work/state"
git -C "$test_root/work" commit -am concurrent >/dev/null
concurrent_sha="$(git -C "$test_root/work" rev-parse HEAD)"
git -C "$test_root/work" switch main >/dev/null
git init --bare "$test_root/concurrent.git" >/dev/null
git -C "$test_root/work" push \
  "$test_root/concurrent.git" \
  "$first_sha:refs/heads/main" \
  "$concurrent_sha:refs/heads/concurrent-source" >/dev/null
cat > "$test_root/concurrent.git/hooks/post-receive" <<'EOF'
#!/bin/sh
while read -r old_sha _new_sha ref; do
  if [ "$ref" = "refs/heads/main" ]; then
    git update-ref refs/heads/main refs/heads/concurrent-source
    exit 0
  fi
done
exit 0
EOF
chmod +x "$test_root/concurrent.git/hooks/post-receive"
if run_sync gitlab "$test_root/concurrent.git" v1.0.0 main true >/dev/null 2>&1; then
  fail_test "a concurrent branch update was overwritten"
fi
assert_ref "$test_root/concurrent.git" refs/heads/main "$concurrent_sha"
if git --git-dir="$test_root/concurrent.git" show-ref --verify --quiet refs/tags/v1.0.0; then
  fail_test "tag was pushed after a concurrent branch update"
fi

# Prerelease tags can be synchronized from the configured release branch.
git -C "$test_root/work" switch -c release/cogfoundry-v0.2.0 >/dev/null
printf '%s\n' release > "$test_root/work/state"
git -C "$test_root/work" commit -am release >/dev/null
release_branch_sha="$(git -C "$test_root/work" rev-parse HEAD)"
git -C "$test_root/work" tag v1.1.0-beta.1
git -C "$test_root/work" push origin release/cogfoundry-v0.2.0 v1.1.0-beta.1 >/dev/null
git init --bare "$test_root/prerelease.git" >/dev/null
run_sync gitee "$test_root/prerelease.git" v1.1.0-beta.1 release/cogfoundry-v0.2.0 >/dev/null
assert_ref "$test_root/prerelease.git" refs/heads/release/cogfoundry-v0.2.0 "$release_branch_sha"
assert_ref "$test_root/prerelease.git" refs/tags/v1.1.0-beta.1 "$release_branch_sha"

# Annotated tags retain their tag object while the branch points at the commit.
printf '%s\n' annotated > "$test_root/work/state"
git -C "$test_root/work" commit -am annotated >/dev/null
annotated_commit="$(git -C "$test_root/work" rev-parse HEAD)"
git -C "$test_root/work" tag -a v1.1.0-rc.1 -m annotated
annotated_object="$(git -C "$test_root/work" rev-parse refs/tags/v1.1.0-rc.1)"
git -C "$test_root/work" push origin release/cogfoundry-v0.2.0 v1.1.0-rc.1 >/dev/null
git init --bare "$test_root/annotated.git" >/dev/null
run_sync gitlab "$test_root/annotated.git" v1.1.0-rc.1 release/cogfoundry-v0.2.0 >/dev/null
assert_ref "$test_root/annotated.git" refs/heads/release/cogfoundry-v0.2.0 "$annotated_commit"
assert_ref "$test_root/annotated.git" refs/tags/v1.1.0-rc.1 "$annotated_object"

# An annotated mirror tag that resolves to a different commit is still a
# conflict. Reject it before changing either the published tag or its branch.
git clone "$test_root/origin.git" "$test_root/annotated-conflict-work" >/dev/null
git -C "$test_root/annotated-conflict-work" config user.name test
git -C "$test_root/annotated-conflict-work" config user.email test@example.com
git -C "$test_root/annotated-conflict-work" tag -d v1.1.0-rc.1 >/dev/null
git -C "$test_root/annotated-conflict-work" tag \
  -a v1.1.0-rc.1 \
  -m conflicting \
  "$release_branch_sha"
conflicting_annotated_object="$(
  git -C "$test_root/annotated-conflict-work" rev-parse refs/tags/v1.1.0-rc.1
)"
git init --bare "$test_root/annotated-conflict.git" >/dev/null
git -C "$test_root/annotated-conflict-work" push \
  "$test_root/annotated-conflict.git" \
  "$release_branch_sha:refs/heads/release/cogfoundry-v0.2.0" \
  refs/tags/v1.1.0-rc.1:refs/tags/v1.1.0-rc.1 >/dev/null
if run_sync \
  gitlab \
  "$test_root/annotated-conflict.git" \
  v1.1.0-rc.1 \
  release/cogfoundry-v0.2.0 >/dev/null 2>&1; then
  fail_test "an annotated remote tag at a different commit was accepted"
fi
assert_ref \
  "$test_root/annotated-conflict.git" \
  refs/tags/v1.1.0-rc.1 \
  "$conflicting_annotated_object"
assert_ref \
  "$test_root/annotated-conflict.git" \
  refs/heads/release/cogfoundry-v0.2.0 \
  "$release_branch_sha"

# A mirror may normalize an annotated source tag into a lightweight tag. The
# tag remains immutable when both forms resolve to the same release commit.
git init --bare "$test_root/normalized-lightweight.git" >/dev/null
git -C "$test_root/work" push \
  "$test_root/normalized-lightweight.git" \
  "$annotated_commit:refs/heads/release/cogfoundry-v0.2.0" \
  "$annotated_commit:refs/tags/v1.1.0-rc.1" >/dev/null
run_sync \
  gitlab \
  "$test_root/normalized-lightweight.git" \
  v1.1.0-rc.1 \
  release/cogfoundry-v0.2.0 >/dev/null
assert_ref \
  "$test_root/normalized-lightweight.git" \
  refs/tags/v1.1.0-rc.1 \
  "$annotated_commit"

# The inverse form is also equivalent: a lightweight source tag can match an
# annotated mirror tag when both resolve to the same release commit.
git -C "$test_root/work" tag -a v1.1.0-rc.2 -m annotated
reverse_annotated_object="$(git -C "$test_root/work" rev-parse refs/tags/v1.1.0-rc.2)"
git -C "$test_root/work" tag -d v1.1.0-rc.2 >/dev/null
git -C "$test_root/work" tag v1.1.0-rc.2
git -C "$test_root/work" push origin v1.1.0-rc.2 >/dev/null
git init --bare "$test_root/normalized-annotated.git" >/dev/null
git -C "$test_root/work" push \
  "$test_root/normalized-annotated.git" \
  "$annotated_commit:refs/heads/release/cogfoundry-v0.2.0" \
  "$reverse_annotated_object:refs/tags/v1.1.0-rc.2" >/dev/null
run_sync \
  gitee \
  "$test_root/normalized-annotated.git" \
  v1.1.0-rc.2 \
  release/cogfoundry-v0.2.0 >/dev/null
assert_ref \
  "$test_root/normalized-annotated.git" \
  refs/tags/v1.1.0-rc.2 \
  "$reverse_annotated_object"

# Provider credentials are mandatory, including when a local test URL is used.
if (
  cd "$test_root/work"
  RELEASE_SYNC_REMOTE_URL="$test_root/gitlab.git" \
    RELEASE_SYNC_MAX_ATTEMPTS=1 \
    "$syncer" \
      --provider gitlab \
      --repo cogfoundry/loomloom \
      --branch main \
      --tag v1.0.0 >/dev/null 2>&1
); then
  fail_test "missing GitLab token was accepted"
fi

echo "release ref synchronization tests passed"
