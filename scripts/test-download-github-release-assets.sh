#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
downloader="$repo_root/scripts/download-github-release-assets.sh"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

release_dir="$tmp_dir/release-source"
mkdir -p "$release_dir"
printf 'test release asset\n' > "$release_dir/loomloom-test.tar.gz"
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$release_dir" && sha256sum loomloom-test.tar.gz > checksums.txt)
else
  (cd "$release_dir" && shasum -a 256 loomloom-test.tar.gz > checksums.txt)
fi

output_dir="$tmp_dir/output"
GITHUB_RELEASE_DOWNLOAD_BASE_URL="file://$release_dir" \
  "$downloader" \
    --repo cogfoundry-labs/loomloom \
    --tag v0.2.0 \
    --output-dir "$output_dir"

cmp "$release_dir/checksums.txt" "$output_dir/checksums.txt"
cmp "$release_dir/loomloom-test.tar.gz" "$output_dir/loomloom-test.tar.gz"

if GITHUB_RELEASE_DOWNLOAD_BASE_URL="file://$release_dir" \
  "$downloader" --repo invalid --tag v0.2.0 --output-dir "$tmp_dir/invalid-repo"; then
  echo "downloader accepted an invalid repository" >&2
  exit 1
fi

if GITHUB_RELEASE_DOWNLOAD_BASE_URL="file://$release_dir" \
  "$downloader" --repo cogfoundry-labs/loomloom --tag latest --output-dir "$tmp_dir/invalid-tag"; then
  echo "downloader accepted an invalid tag" >&2
  exit 1
fi

unsafe_dir="$tmp_dir/unsafe-release"
mkdir -p "$unsafe_dir"
printf '%064d  ../escape\n' 0 > "$unsafe_dir/checksums.txt"
if GITHUB_RELEASE_DOWNLOAD_BASE_URL="file://$unsafe_dir" \
  "$downloader" \
    --repo cogfoundry-labs/loomloom \
    --tag v0.2.0 \
    --output-dir "$tmp_dir/unsafe-output"; then
  echo "downloader accepted an unsafe release asset name" >&2
  exit 1
fi

echo "GitHub release asset downloader tests passed"
