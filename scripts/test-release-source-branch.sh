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

commit() {
  printf '%s\n' "$1" > "$test_root/work/state"
  git -C "$test_root/work" add state
  git -C "$test_root/work" commit -m "$1" >/dev/null
}
resolve() {
  (cd "$test_root/work" && "$resolver" "$@")
}
expect() {
  local want="$1"
  shift
  local got
  got="$(resolve "$@")"
  if [[ "$got" != "$want" ]]; then
    echo "resolve $*: expected $want, got $got" >&2
    exit 1
  fi
}
reject() {
  if resolve "$@" >/dev/null 2>&1; then
    echo "resolve $* was accepted" >&2
    exit 1
  fi
}

# A minor version is cut from main: beta first, then stable on the same commit.
commit feature
git -C "$test_root/work" tag v1.0.0-beta.1
git -C "$test_root/work" push origin main v1.0.0-beta.1 >/dev/null
expect main v1.0.0-beta.1
reject v1.0.0
git -C "$test_root/work" tag v1.0.0
git -C "$test_root/work" push origin v1.0.0 >/dev/null
expect main v1.0.0

# The stable tag stays valid after main moves on, because it reuses the beta commit.
commit next-feature
git -C "$test_root/work" push origin main >/dev/null
expect main v1.0.0
reject v1.0.0-beta.1
expect main v1.0.0-beta.1 --allow-history

# A stable tag must match its highest beta, not an earlier one.
git -C "$test_root/work" tag v1.1.0-beta.1
commit later-beta
git -C "$test_root/work" tag v1.1.0-beta.2
git -C "$test_root/work" tag v1.1.0 v1.1.0-beta.1
git -C "$test_root/work" push origin main v1.1.0-beta.1 v1.1.0-beta.2 v1.1.0 >/dev/null
reject v1.1.0

# A patch release comes from release/X.Y, never from main.
git -C "$test_root/work" switch -c release/1.0 v1.0.0 >/dev/null
commit cherry-picked-fix
git -C "$test_root/work" tag v1.0.1-beta.1
git -C "$test_root/work" tag v1.0.1
git -C "$test_root/work" push origin release/1.0 v1.0.1-beta.1 v1.0.1 >/dev/null
expect release/1.0 v1.0.1-beta.1
expect release/1.0 v1.0.1
git -C "$test_root/work" switch main >/dev/null
git -C "$test_root/work" tag v1.1.1-beta.1
git -C "$test_root/work" push origin v1.1.1-beta.1 >/dev/null
reject v1.1.1-beta.1

# Only vX.Y.Z and vX.Y.Z-beta.N are release tags.
git -C "$test_root/work" tag v1.2.0-internal.1
git -C "$test_root/work" tag v1.2.0-rc.1
reject v1.2.0-internal.1
reject v1.2.0-rc.1

echo "release source branch tests passed"
