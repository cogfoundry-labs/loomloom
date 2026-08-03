#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
resolver="$repo_root/scripts/resolve-release-source-branch.sh"
test_root="$(mktemp -d)"
trap 'rm -rf "$test_root"' EXIT

git init --bare "$test_root/remote.git" >/dev/null
git init -b main "$test_root/work" >/dev/null
git -C "$test_root/work" config user.name test
git -C "$test_root/work" config user.email test@example.com
git -C "$test_root/work" remote add origin "$test_root/remote.git"

printf '%s\n' main > "$test_root/work/state"
git -C "$test_root/work" add state
git -C "$test_root/work" commit -m main >/dev/null
git -C "$test_root/work" tag v1.0.0
git -C "$test_root/work" push origin main v1.0.0 >/dev/null

stable_branch="$(cd "$test_root/work" && "$resolver" v1.0.0)"
if [[ "$stable_branch" != "main" ]]; then
  echo "stable branch resolution failed: $stable_branch" >&2
  exit 1
fi

git -C "$test_root/work" switch -c release/cogfoundry-v0.2.0 >/dev/null
printf '%s\n' release > "$test_root/work/state"
git -C "$test_root/work" commit -am release >/dev/null
git -C "$test_root/work" tag v1.1.0-internal.1
git -C "$test_root/work" push origin release/cogfoundry-v0.2.0 v1.1.0-internal.1 >/dev/null

prerelease_branch="$(cd "$test_root/work" && "$resolver" v1.1.0-internal.1)"
if [[ "$prerelease_branch" != "release/cogfoundry-v0.2.0" ]]; then
  echo "prerelease branch resolution failed: $prerelease_branch" >&2
  exit 1
fi

git -C "$test_root/work" switch main >/dev/null
printf '%s\n' old > "$test_root/work/state"
git -C "$test_root/work" commit -am old >/dev/null
git -C "$test_root/work" tag v1.0.1
printf '%s\n' current > "$test_root/work/state"
git -C "$test_root/work" commit -am current >/dev/null
git -C "$test_root/work" push origin main v1.0.1 >/dev/null

if (cd "$test_root/work" && "$resolver" v1.0.1 >/dev/null 2>&1); then
  echo "old stable tag was accepted" >&2
  exit 1
fi

historical_stable_branch="$(cd "$test_root/work" && "$resolver" v1.0.1 --allow-history)"
if [[ "$historical_stable_branch" != "main" ]]; then
  echo "historical stable branch resolution failed: $historical_stable_branch" >&2
  exit 1
fi

# A lower semantic version is valid when it points at the current branch HEAD.
git -C "$test_root/work" tag v0.5.0
git -C "$test_root/work" push origin v0.5.0 >/dev/null
lower_version_branch="$(cd "$test_root/work" && "$resolver" v0.5.0)"
if [[ "$lower_version_branch" != "main" ]]; then
  echo "lower stable version was rejected: $lower_version_branch" >&2
  exit 1
fi

# A prerelease at main HEAD resolves to main, then remains repairable as history.
printf '%s\n' prerelease-main > "$test_root/work/state"
git -C "$test_root/work" commit -am prerelease-main >/dev/null
git -C "$test_root/work" tag v2.0.0-beta.1
git -C "$test_root/work" push origin main v2.0.0-beta.1 >/dev/null
main_prerelease_branch="$(cd "$test_root/work" && "$resolver" v2.0.0-beta.1)"
if [[ "$main_prerelease_branch" != "main" ]]; then
  echo "main prerelease branch resolution failed: $main_prerelease_branch" >&2
  exit 1
fi

printf '%s\n' after-prerelease > "$test_root/work/state"
git -C "$test_root/work" commit -am after-prerelease >/dev/null
git -C "$test_root/work" push origin main >/dev/null
historical_prerelease_branch="$(cd "$test_root/work" && "$resolver" v2.0.0-beta.1 --allow-history)"
if [[ "$historical_prerelease_branch" != "main" ]]; then
  echo "historical main prerelease resolution failed: $historical_prerelease_branch" >&2
  exit 1
fi

# A prerelease remains repairable from the release branch after that branch advances.
git -C "$test_root/work" switch release/cogfoundry-v0.2.0 >/dev/null
printf '%s\n' release-advanced > "$test_root/work/state"
git -C "$test_root/work" commit -am release-advanced >/dev/null
git -C "$test_root/work" push origin release/cogfoundry-v0.2.0 >/dev/null
if (cd "$test_root/work" && "$resolver" v1.1.0-internal.1 >/dev/null 2>&1); then
  echo "historical release prerelease was accepted without --allow-history" >&2
  exit 1
fi
historical_release_branch="$(cd "$test_root/work" && "$resolver" v1.1.0-internal.1 --allow-history)"
if [[ "$historical_release_branch" != "release/cogfoundry-v0.2.0" ]]; then
  echo "historical release prerelease resolution failed: $historical_release_branch" >&2
  exit 1
fi

echo "release source branch tests passed"
