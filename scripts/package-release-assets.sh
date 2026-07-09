#!/usr/bin/env bash
set -euo pipefail

VERSION="${VERSION:-}"
OUT_DIR="${OUT_DIR:-release}"

usage() {
  cat <<'EOF'
Usage: scripts/package-release-assets.sh --version <tag> [--out-dir <path>]

Build and package LoomLoom CLI release assets.

Options:
  --version <tag>    Version injected into the CLI binary, for example v0.1.0-beta.1
  --out-dir <path>   Output directory for release assets (default: release)
  --help             Show this help text
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      VERSION="${2:-}"
      shift 2
      ;;
    --out-dir)
      OUT_DIR="${2:-release}"
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

if [[ -z "$VERSION" ]]; then
  echo "--version is required" >&2
  exit 1
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ "$OUT_DIR" == /* ]]; then
  out_dir="$OUT_DIR"
else
  out_dir="$repo_root/$OUT_DIR"
fi
dist_dir="$repo_root/dist"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

checksum_cmd() {
  if command -v sha256sum >/dev/null 2>&1; then
    printf '%s\n' "sha256sum"
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    printf '%s\n' "shasum -a 256"
    return
  fi
  echo "missing required command: sha256sum or shasum" >&2
  exit 1
}

require_cmd go
require_cmd tar
require_cmd zip

zip_file() {
  local archive_path="$1"
  local source_dir="$2"
  shift 2
  (cd "$source_dir" && zip -qr "$archive_path" "$@")
}

rm -rf "$dist_dir" "$out_dir"
mkdir -p "$dist_dir" "$out_dir"

build_cli() {
  local goos="$1"
  local goarch="$2"
  local output_path="$dist_dir/loomloom-${goos}-${goarch}"
  if [[ "$goos" == "windows" ]]; then
    output_path="${output_path}.exe"
  fi

  echo "building CLI: ${goos}/${goarch}"
  (
    cd "$repo_root/cli"
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" GOWORK=off \
      go build \
        -buildvcs=false \
        -ldflags "-X github.com/Cogfoundry-ai/loomloom/cli/internal/version.Version=${VERSION}" \
        -o "$output_path" \
        ./cmd/loomloom
  )
}

package_binary() {
  local binary="$1"
  local name
  local staging
  name="$(basename "$binary")"
  echo "packaging binary: $name"
  staging="$(mktemp -d)"
  if [[ "$name" == *.exe ]]; then
    cp "$binary" "$staging/loomloom.exe"
    zip_file "$staging/release.zip" "$staging" loomloom.exe
    mv "$staging/release.zip" "$out_dir/${name%.exe}.zip"
  else
    cp "$binary" "$staging/loomloom"
    tar -C "$staging" -czf "$out_dir/${name}.tar.gz" loomloom
  fi
  rm -rf "$staging"
}

for target in \
  linux/amd64 \
  linux/arm64 \
  darwin/amd64 \
  darwin/arm64 \
  windows/amd64 \
  windows/arm64
do
  build_cli "${target%/*}" "${target#*/}"
done

while IFS= read -r binary; do
  package_binary "$binary"
done < <(find "$dist_dir" -type f -name 'loomloom-*' | sort)

echo "packaging skills"
tar -C "$repo_root" -czf "$out_dir/loomloom-skills.tar.gz" skills
zip_file "$out_dir/loomloom-skills.zip" "$repo_root" skills
cp "$repo_root"/install.sh "$repo_root"/install-gitee.sh "$repo_root"/install.ps1 \
  "$repo_root"/uninstall.sh "$repo_root"/uninstall.ps1 \
  "$repo_root"/manifest.json "$repo_root"/README.md "$out_dir"/

(cd "$out_dir" && $(checksum_cmd) -- * > checksums.txt)

echo "release assets written to $out_dir"
