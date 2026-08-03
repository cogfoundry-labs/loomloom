#!/usr/bin/env bash
set -euo pipefail

REPOSITORY=""
TAG=""
OUTPUT_DIR=""

usage() {
  cat <<'EOF'
Usage: scripts/download-github-release-assets.sh --repo <owner/repo> --tag <tag> --output-dir <directory>

Download and verify a complete LoomLoom GitHub Release through its public,
stable release URLs. This avoids the GitHub release-assets API used by
`gh release download`, which can return HTTP 500 even when public downloads
are healthy.

Environment:
  GITHUB_RELEASE_DOWNLOAD_BASE_URL  Optional exact release download base URL
                                    used by local tests.
EOF
}

fail() {
  echo "$*" >&2
  exit 1
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
    --output-dir)
      OUTPUT_DIR="${2:-}"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      fail "unknown argument: $1"
      ;;
  esac
done

[[ "$REPOSITORY" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || \
  fail "--repo must use owner/repo format"
[[ "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-(internal|beta|rc)\.[0-9]+)?$ ]] || \
  fail "unsupported release tag: $TAG"
[[ -n "$OUTPUT_DIR" ]] || fail "--output-dir is required"
command -v curl >/dev/null 2>&1 || fail "missing required command: curl"

if [[ -e "$OUTPUT_DIR" && ! -d "$OUTPUT_DIR" ]]; then
  fail "output path is not a directory: $OUTPUT_DIR"
fi
mkdir -p "$OUTPUT_DIR"
if [[ -n "$(find "$OUTPUT_DIR" -mindepth 1 -maxdepth 1 -print -quit)" ]]; then
  fail "output directory must be empty: $OUTPUT_DIR"
fi

if [[ -n "${GITHUB_RELEASE_DOWNLOAD_BASE_URL:-}" ]]; then
  base_url="${GITHUB_RELEASE_DOWNLOAD_BASE_URL%/}"
else
  base_url="https://github.com/${REPOSITORY}/releases/download/${TAG}"
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

download() {
  local filename="$1"
  echo "downloading GitHub release asset: $filename"
  curl \
    --fail \
    --show-error \
    --location \
    --retry 5 \
    --retry-all-errors \
    --connect-timeout 15 \
    --output "$tmp_dir/$filename" \
    "$base_url/$filename"
}

download checksums.txt

asset_count=0
declare -a asset_names=()
while IFS= read -r line || [[ -n "$line" ]]; do
  [[ -n "$line" ]] || continue

  checksum="${line%% *}"
  filename="${line#"$checksum"}"
  filename="${filename# }"
  filename="${filename# }"
  filename="${filename#\*}"

  [[ "$checksum" =~ ^[0-9a-f]{64}$ ]] || fail "invalid checksum entry: $line"
  [[ "$filename" =~ ^[A-Za-z0-9._-]+$ ]] || \
    fail "invalid release asset name in checksums.txt: $filename"
  [[ "$filename" != "." && "$filename" != ".." && "$filename" != "checksums.txt" ]] || \
    fail "invalid release asset name in checksums.txt: $filename"

  for existing_name in "${asset_names[@]:-}"; do
    [[ "$existing_name" != "$filename" ]] || \
      fail "duplicate release asset in checksums.txt: $filename"
  done
  asset_names+=("$filename")
  asset_count=$((asset_count + 1))
done < "$tmp_dir/checksums.txt"

[[ "$asset_count" -gt 0 ]] || fail "checksums.txt does not list any release assets"

for filename in "${asset_names[@]}"; do
  download "$filename"
done

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
"$repo_root/scripts/validate-release-assets.sh" "$tmp_dir"

find "$tmp_dir" -maxdepth 1 -type f -exec mv {} "$OUTPUT_DIR/" \;
echo "downloaded and verified $asset_count GitHub release assets in $OUTPUT_DIR"
