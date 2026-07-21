#!/usr/bin/env bash
set -euo pipefail

REPOSITORY=""
TAG=""
ASSETS_DIR="release"
PRERELEASE="false"
API_BASE="${GITEE_API_BASE:-https://gitee.com/api/v5}"

usage() {
  cat <<'EOF'
Usage: scripts/publish-gitee-release.sh --repo <owner/repo> --tag <tag> [options]

Create or refresh a Gitee Release with an existing directory of release assets.

Options:
  --repo <owner/repo>       Gitee repository
  --tag <tag>               Existing Git tag for the release
  --assets-dir <path>       Directory containing release assets (default: release)
  --prerelease <true|false> Whether to mark a newly created release as prerelease
  --help                    Show this help text

Environment:
  GITEE_SYNC_TOKEN          Token with repository and Release API write access
  GITEE_API_BASE            Optional API base override for testing
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
    --assets-dir)
      ASSETS_DIR="${2:-}"
      shift 2
      ;;
    --prerelease)
      PRERELEASE="${2:-}"
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
if [[ "$PRERELEASE" != "true" && "$PRERELEASE" != "false" ]]; then
  echo "--prerelease must be true or false" >&2
  exit 1
fi
if [[ ! -d "$ASSETS_DIR" || ! -f "$ASSETS_DIR/checksums.txt" ]]; then
  echo "assets directory must contain checksums.txt: $ASSETS_DIR" >&2
  exit 1
fi
if [[ -z "${GITEE_SYNC_TOKEN:-}" ]]; then
  echo "GITEE_SYNC_TOKEN is required" >&2
  exit 1
fi

for command in curl jq; do
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "missing required command: $command" >&2
    exit 1
  fi
done

if command -v sha256sum >/dev/null 2>&1; then
  CHECKSUM_COMMAND=(sha256sum)
elif command -v shasum >/dev/null 2>&1; then
  CHECKSUM_COMMAND=(shasum -a 256)
else
  echo "missing required command: sha256sum or shasum" >&2
  exit 1
fi

asset_count=0
while IFS= read -r -d '' asset; do
  asset_name="$(basename "$asset")"
  expected="$(awk -v name="$asset_name" '$2 == name { print $1 }' "$ASSETS_DIR/checksums.txt")"
  actual="$("${CHECKSUM_COMMAND[@]}" "$asset" | awk '{print $1}')"
  if [[ -z "$expected" || "$expected" != "$actual" ]]; then
    echo "checksum validation failed for release asset: $asset_name" >&2
    exit 1
  fi
  asset_count=$((asset_count + 1))
done < <(find "$ASSETS_DIR" -maxdepth 1 -type f ! -name checksums.txt -print0 | sort -z)

if [[ "$asset_count" -eq 0 ]]; then
  echo "no release assets found in $ASSETS_DIR" >&2
  exit 1
fi

OWNER="${REPOSITORY%/*}"
REPO="${REPOSITORY#*/}"
RELEASES_URL="${API_BASE}/repos/${OWNER}/${REPO}/releases"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

release_file="$TMP_DIR/release.json"
status="$(
  curl -sS \
    --get \
    --output "$release_file" \
    --write-out '%{http_code}' \
    --data-urlencode "access_token=${GITEE_SYNC_TOKEN}" \
    "${RELEASES_URL}/tags/${TAG}"
)"

create_release() {
  echo "creating Gitee release: $TAG"
  curl -fsS \
    --request POST \
    --output "$release_file" \
    --form "access_token=${GITEE_SYNC_TOKEN}" \
    --form "tag_name=${TAG}" \
    --form "name=${TAG}" \
    --form "body=LoomLoom ${TAG}" \
    --form "prerelease=${PRERELEASE}" \
    "$RELEASES_URL"
}

case "$status" in
  200)
    if jq -e 'type == "object" and (.id != null)' "$release_file" >/dev/null; then
      echo "refreshing existing Gitee release: $TAG"
    elif jq -e '. == null' "$release_file" >/dev/null; then
      # Gitee returns HTTP 200 with a JSON null body when the tag exists but
      # no Release has been created for it yet.
      create_release
    else
      echo "Gitee returned an invalid release response for $TAG (HTTP 200)" >&2
      jq -c . "$release_file" >&2 2>/dev/null || true
      exit 1
    fi
    ;;
  404)
    create_release
    ;;
  *)
    echo "failed to query Gitee release $TAG (HTTP $status)" >&2
    jq -c . "$release_file" >&2 2>/dev/null || true
    exit 1
    ;;
esac

if ! release_id="$(jq -er 'if type == "object" and (.id != null) then .id else empty end' "$release_file")"; then
  echo "Gitee release response does not contain an id for $TAG" >&2
  jq -c . "$release_file" >&2 2>/dev/null || true
  exit 1
fi
attachments_file="$TMP_DIR/attachments.json"
curl -fsS \
  --get \
  --output "$attachments_file" \
  --data-urlencode "access_token=${GITEE_SYNC_TOKEN}" \
  --data-urlencode "per_page=100" \
  "${RELEASES_URL}/${release_id}/attach_files"

missing_assets_file="$TMP_DIR/missing-assets"
: > "$missing_assets_file"

verify_existing_asset() {
  local asset="$1"
  local asset_name
  local attachment_count
  local attachment_id
  local local_checksum
  local remote_asset
  local remote_checksum
  asset_name="$(basename "$asset")"

  attachment_count="$(
    jq -r --arg name "$asset_name" '[.[] | select(.name == $name)] | length' "$attachments_file"
  )"
  if [[ "$attachment_count" -gt 1 ]]; then
    echo "multiple Gitee release assets use the same name: $asset_name" >&2
    exit 1
  fi

  if [[ "$attachment_count" -eq 1 ]]; then
    attachment_id="$(
      jq -r --arg name "$asset_name" '.[] | select(.name == $name) | .id' "$attachments_file"
    )"
    remote_asset="$TMP_DIR/remote-${attachment_id}"
    curl -fsSL \
      --get \
      --output "$remote_asset" \
      --data-urlencode "access_token=${GITEE_SYNC_TOKEN}" \
      "${RELEASES_URL}/${release_id}/attach_files/${attachment_id}/download"
    local_checksum="$("${CHECKSUM_COMMAND[@]}" "$asset" | awk '{print $1}')"
    remote_checksum="$("${CHECKSUM_COMMAND[@]}" "$remote_asset" | awk '{print $1}')"

    if [[ "$local_checksum" != "$remote_checksum" ]]; then
      echo "published Gitee release asset differs from GitHub: $asset_name" >&2
      echo "published release assets are immutable; create a new tag instead" >&2
      exit 1
    fi

    echo "keeping identical Gitee release asset: $asset_name"
    skipped_count=$((skipped_count + 1))
    return
  fi

  printf '%s\0' "$asset" >> "$missing_assets_file"
}

upload_missing_asset() {
  local asset="$1"
  local asset_name
  asset_name="$(basename "$asset")"
  echo "uploading Gitee release asset: $asset_name"
  curl -fsS \
    --request POST \
    --form "access_token=${GITEE_SYNC_TOKEN}" \
    --form "file=@${asset}" \
    "${RELEASES_URL}/${release_id}/attach_files" \
    >/dev/null
  uploaded_count=$((uploaded_count + 1))
}

uploaded_count=0
skipped_count=0
while IFS= read -r -d '' asset; do
  verify_existing_asset "$asset"
done < <(find "$ASSETS_DIR" -maxdepth 1 -type f ! -name checksums.txt -print0 | sort -z)
verify_existing_asset "$ASSETS_DIR/checksums.txt"

while IFS= read -r -d '' asset; do
  upload_missing_asset "$asset"
done < "$missing_assets_file"

echo "published $uploaded_count missing assets and kept $skipped_count identical assets for $TAG"
