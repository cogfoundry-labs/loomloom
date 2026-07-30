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
docs_script="$repo_root/scripts/template-spec-docs.sh"
references_script="$repo_root/scripts/skill-references.sh"
uninstall_test_script="$repo_root/scripts/test-uninstall.sh"
verification_dir=""

cleanup() {
  "$references_script" clean
  "$docs_script" clean
  if [[ -n "$verification_dir" ]]; then
    rm -rf "$verification_dir"
  fi
}

trap cleanup EXIT

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
require_cmd gzip
require_cmd tar
require_cmd zip

"$docs_script" prepare-checked
"$references_script" prepare-checked
"$uninstall_test_script"

create_archive() (
  local format="$1"
  local archive_path="$2"
  local source_dir="$3"
  local staging
  local file_list
  local tar_file
  shift 3

  staging="$(mktemp -d)"
  file_list="$(mktemp)"
  tar_file="$(mktemp)"
  trap 'rm -rf "$staging" "$file_list" "$tar_file"' EXIT

  for entry in "$@"; do
    mkdir -p "$staging/$(dirname "$entry")"
    cp -R "$source_dir/$entry" "$staging/$entry"
  done

  while IFS= read -r path; do
    if [[ -x "$path" ]]; then
      chmod 0755 "$path"
    else
      chmod 0644 "$path"
    fi
    TZ=UTC touch -t 200001010000 "$path"
  done < <(find "$staging" -type f | LC_ALL=C sort)
  while IFS= read -r path; do
    chmod 0755 "$path"
    TZ=UTC touch -t 200001010000 "$path"
  done < <(find "$staging" -type d | LC_ALL=C sort -r)

  (
    cd "$staging"
    find . ! -type d -print | LC_ALL=C sort | sed 's#^\./##' > "$file_list"
  )
  rm -f "$archive_path"

  case "$format" in
    tar.gz)
      if tar --version 2>&1 | grep -q 'GNU tar'; then
        COPYFILE_DISABLE=1 tar \
          --format=ustar \
          --owner=0 \
          --group=0 \
          --numeric-owner \
          -C "$staging" \
          -cf "$tar_file" \
          -T "$file_list"
      else
        COPYFILE_DISABLE=1 tar \
          --format ustar \
          --uid 0 \
          --gid 0 \
          --uname root \
          --gname root \
          -C "$staging" \
          -cf "$tar_file" \
          -T "$file_list"
      fi
      gzip -n -9 -c "$tar_file" > "$archive_path"
      ;;
    zip)
      (
        cd "$staging"
        COPYFILE_DISABLE=1 zip -X -q "$archive_path" -@ < "$file_list"
      )
      ;;
    *)
      echo "unsupported archive format: $format" >&2
      exit 1
      ;;
  esac
)

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

verify_bundled_docs() {
  verification_dir="$(mktemp -d)"
  local binary="$verification_dir/loomloom"

  echo "verifying bundled TemplateSpec docs"
  (
    cd "$repo_root/cli"
    GOWORK=off go build \
      -buildvcs=false \
      -ldflags "-X github.com/Cogfoundry-ai/loomloom/cli/internal/version.Version=${VERSION}" \
      -o "$binary" \
      ./cmd/loomloom
  )
  "$binary" template-spec docs spec >/dev/null
  "$binary" template-spec docs spec --lang zh-CN >/dev/null
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
    create_archive zip "$staging/release.zip" "$staging" loomloom.exe
    mv "$staging/release.zip" "$out_dir/${name%.exe}.zip"
  else
    cp "$binary" "$staging/loomloom"
    create_archive tar.gz "$out_dir/${name}.tar.gz" "$staging" loomloom
  fi
  rm -rf "$staging"
}

verify_bundled_docs

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
create_archive tar.gz "$out_dir/loomloom-skills.tar.gz" "$repo_root" skills
create_archive zip "$out_dir/loomloom-skills.zip" "$repo_root" skills
cp "$repo_root"/install.sh "$repo_root"/install-gitee.sh "$repo_root"/install.ps1 \
  "$repo_root"/uninstall.sh "$repo_root"/uninstall.ps1 \
  "$repo_root"/manifest.json "$repo_root"/README.md "$out_dir"/

(cd "$out_dir" && $(checksum_cmd) -- * > checksums.txt)

echo "release assets written to $out_dir"
