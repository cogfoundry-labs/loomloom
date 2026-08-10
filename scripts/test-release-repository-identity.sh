#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
expected_repo="cogfoundry-labs/loomloom"
expected_tap="cogfoundry-labs/tap"
expected_tap_repo="cogfoundry-labs/homebrew-tap"

assert_contains() {
  local file="$1"
  local expected="$2"
  if ! grep -Fq "$expected" "$repo_root/$file"; then
    echo "$file does not contain expected repository identity: $expected" >&2
    exit 1
  fi
}

assert_not_contains() {
  local file="$1"
  local unexpected="$2"
  if grep -Fq "$unexpected" "$repo_root/$file"; then
    echo "$file still contains legacy repository identity: $unexpected" >&2
    exit 1
  fi
}

assert_contains scripts/install.sh "GITHUB_REPO=\"$expected_repo\""
assert_contains scripts/install.sh "LOOMLOOM_HOMEBREW_TAP:-$expected_tap"
assert_contains scripts/install.ps1 "\$GithubRepo = \"$expected_repo\""
assert_contains src/cli/internal/version/version.go "repoOwnerRepo      = \"$expected_repo\""
assert_contains .github/workflows/release.yml "github.com/$expected_tap_repo.git"
assert_contains .github/workflows/release.yml 'mkdir -p "$tap_dir/Formula"'
assert_contains .workflow/loomloom-cli-release.yml 'scripts/resolve-release-source-branch.sh "$tag_name"'
assert_contains agent-guidance/loomloom/references/setup.md "raw.githubusercontent.com/$expected_repo/main/scripts/install.sh"

for file in \
  scripts/install.sh \
  scripts/install.ps1 \
  src/cli/internal/version/version.go \
  .github/workflows/release.yml \
  agent-guidance/loomloom/references/setup.md
do
  assert_not_contains "$file" "Cogfoundry-ai"
done

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT
checksum="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
for asset in \
  loomloom-darwin-arm64.tar.gz \
  loomloom-darwin-amd64.tar.gz \
  loomloom-linux-arm64.tar.gz \
  loomloom-linux-amd64.tar.gz
do
  printf '%s  %s\n' "$checksum" "$asset" >> "$tmp_dir/checksums.txt"
done

"$repo_root/scripts/render-homebrew-formula.sh" \
  --tag v1.2.3 \
  --checksums "$tmp_dir/checksums.txt" \
  > "$tmp_dir/loomloom.rb"

assert_contains_path="$tmp_dir/loomloom.rb"
if ! grep -Fq "homepage \"https://github.com/$expected_repo\"" "$assert_contains_path"; then
  echo "rendered Homebrew formula has the wrong homepage" >&2
  exit 1
fi
if ! grep -Fq "https://github.com/$expected_repo/releases/download/v1.2.3/" "$assert_contains_path"; then
  echo "rendered Homebrew formula has the wrong release URL" >&2
  exit 1
fi

echo "release repository identity tests passed"
