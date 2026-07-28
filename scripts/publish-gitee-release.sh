#!/usr/bin/env bash
set -euo pipefail

REPOSITORY=""
TAG=""
ASSETS_DIR="release"
API_BASE="${GITEE_API_BASE:-https://gitee.com/api/v5}"

usage() {
  cat <<'EOF'
Usage: scripts/publish-gitee-release.sh --repo <owner/repo> --tag <tag> [options]

Create or refresh a Gitee Release with an existing directory of release assets.

Options:
  --repo <owner/repo>       Gitee repository
  --tag <tag>               Existing stable or prerelease Git tag
  --assets-dir <path>       Directory containing release assets (default: release)
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
  echo "unsupported release tag for Gitee publication: $TAG" >&2
  exit 1
fi
PRERELEASE=false
if [[ "$TAG" =~ -(beta|rc|internal)\.[0-9]+$ ]]; then
  PRERELEASE=true
fi
if [[ ! -d "$ASSETS_DIR" || ! -f "$ASSETS_DIR/checksums.txt" ]]; then
  echo "assets directory must contain checksums.txt: $ASSETS_DIR" >&2
  exit 1
fi
if [[ -z "${GITEE_SYNC_TOKEN:-}" ]]; then
  echo "GITEE_SYNC_TOKEN is required" >&2
  exit 1
fi

for command in curl git jq; do
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
TARGET_COMMITISH="$(git rev-list -n 1 "refs/tags/${TAG}" 2>/dev/null || true)"
if [[ -z "$TARGET_COMMITISH" ]]; then
  echo "unable to resolve release tag: $TAG" >&2
  exit 1
fi
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

CURL_MAX_ATTEMPTS=5
CURL_HTTP_STATUS=""

is_retryable_http_status() {
  case "$1" in
    408|429|5??) return 0 ;;
    *) return 1 ;;
  esac
}

is_retryable_curl_exit() {
  case "$1" in
    5|6|7|18|28|35|47|52|55|56|92) return 0 ;;
    *) return 1 ;;
  esac
}

retry_delay() {
  local attempt="$1"
  sleep $((attempt * 2))
}

curl_request() {
  local output="$1"
  local max_attempts="$2"
  shift 2

  local attempt=1
  local curl_exit=0
  local status=""
  local partial="${output}.partial"

  while true; do
    rm -f "$partial"
    if status="$(
      curl -sS \
        --connect-timeout 20 \
        --max-time 600 \
        --output "$partial" \
        --write-out '%{http_code}' \
        "$@"
    )"; then
      curl_exit=0
      mv "$partial" "$output"
      CURL_HTTP_STATUS="$status"
      if ! is_retryable_http_status "$status" || [[ "$attempt" -ge "$max_attempts" ]]; then
        return 0
      fi
      echo "Gitee API returned HTTP $status; retrying ($attempt/$max_attempts)" >&2
    else
      curl_exit=$?
      CURL_HTTP_STATUS="${status:-000}"
      rm -f "$partial"
      if ! is_retryable_curl_exit "$curl_exit" || [[ "$attempt" -ge "$max_attempts" ]]; then
        return "$curl_exit"
      fi
      echo "Gitee API connection failed with curl exit $curl_exit; retrying ($attempt/$max_attempts)" >&2
    fi

    retry_delay "$attempt"
    attempt=$((attempt + 1))
  done
}

is_success_http_status() {
  [[ "$1" == 2?? ]]
}

query_release() {
  local output="$1"
  curl_request "$output" "$CURL_MAX_ATTEMPTS" \
    --get \
    --data-urlencode "access_token=${GITEE_SYNC_TOKEN}" \
    "${RELEASES_URL}/tags/${TAG}"
}

release_file="$TMP_DIR/release.json"
query_exit=0
if query_release "$release_file"; then
  :
else
  query_exit=$?
  echo "failed to query Gitee release $TAG (curl exit $query_exit)" >&2
  exit 1
fi
status="$CURL_HTTP_STATUS"

create_release() {
  echo "creating Gitee release: $TAG"
  local create_file="$TMP_DIR/create-release.json"
  local create_exit=0
  local create_status=""

  if curl_request "$create_file" 1 \
    --request POST \
    --form "access_token=${GITEE_SYNC_TOKEN}" \
    --form "tag_name=${TAG}" \
    --form "name=${TAG}" \
    --form "body=LoomLoom ${TAG}" \
    --form "target_commitish=${TARGET_COMMITISH}" \
    --form "prerelease=${PRERELEASE}" \
    "$RELEASES_URL"; then
    create_status="$CURL_HTTP_STATUS"
    if is_success_http_status "$create_status"; then
      mv "$create_file" "$release_file"
      return
    fi
  else
    create_exit=$?
  fi

  # A failed POST can still have created the Release before the response was
  # interrupted. Query by tag before deciding that creation failed.
  if query_release "$release_file" && [[ "$CURL_HTTP_STATUS" == "200" ]] && \
    jq -e 'type == "object" and (.id != null)' "$release_file" >/dev/null; then
    echo "Gitee release creation response was interrupted; recovered existing release: $TAG"
    return
  fi

  echo "failed to create Gitee release $TAG (HTTP ${create_status:-000}, curl exit $create_exit)" >&2
  jq -c . "$create_file" >&2 2>/dev/null || true
  exit 1
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
refresh_attachments() {
  local request_exit=0
  if curl_request "$attachments_file" "$CURL_MAX_ATTEMPTS" \
    --get \
    --data-urlencode "access_token=${GITEE_SYNC_TOKEN}" \
    --data-urlencode "per_page=100" \
    "${RELEASES_URL}/${release_id}/attach_files"; then
    :
  else
    request_exit=$?
    echo "failed to query Gitee release assets (curl exit $request_exit)" >&2
    return 1
  fi
  if ! is_success_http_status "$CURL_HTTP_STATUS"; then
    echo "failed to query Gitee release assets (HTTP $CURL_HTTP_STATUS)" >&2
    return 1
  fi
  if ! jq -e 'type == "array"' "$attachments_file" >/dev/null; then
    echo "Gitee returned an invalid release asset list" >&2
    return 1
  fi
}

refresh_attachments

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
    local download_exit=0
    if curl_request "$remote_asset" "$CURL_MAX_ATTEMPTS" \
      --get \
      --location \
      --data-urlencode "access_token=${GITEE_SYNC_TOKEN}" \
      "${RELEASES_URL}/${release_id}/attach_files/${attachment_id}/download"; then
      :
    else
      download_exit=$?
      echo "failed to download Gitee release asset: $asset_name (curl exit $download_exit)" >&2
      exit 1
    fi
    if ! is_success_http_status "$CURL_HTTP_STATUS"; then
      echo "failed to download Gitee release asset: $asset_name (HTTP $CURL_HTTP_STATUS)" >&2
      exit 1
    fi
    local_checksum="$("${CHECKSUM_COMMAND[@]}" "$asset" | awk '{print $1}')"
    remote_checksum="$("${CHECKSUM_COMMAND[@]}" "$remote_asset" | awk '{print $1}')"

    if [[ "$local_checksum" != "$remote_checksum" ]]; then
      echo "published Gitee release asset differs from the current Gitee build: $asset_name" >&2
      echo "published release assets are immutable; create a new tag instead" >&2
      exit 1
    fi

    return 0
  fi

  return 1
}

classify_asset() {
  local asset="$1"
  local asset_name
  asset_name="$(basename "$asset")"

  if verify_existing_asset "$asset"; then
    echo "keeping identical Gitee release asset: $asset_name"
    skipped_count=$((skipped_count + 1))
  else
    printf '%s\0' "$asset" >> "$missing_assets_file"
  fi
}

confirm_uploaded_asset() {
  local asset="$1"
  local check=1

  while [[ "$check" -le "$CURL_MAX_ATTEMPTS" ]]; do
    if ! refresh_attachments; then
      echo "unable to determine whether Gitee stored the uploaded asset; stopping safely" >&2
      exit 1
    fi
    if verify_existing_asset "$asset"; then
      return 0
    fi
    if [[ "$check" -lt "$CURL_MAX_ATTEMPTS" ]]; then
      retry_delay "$check"
    fi
    check=$((check + 1))
  done

  return 1
}

upload_missing_asset() {
  local asset="$1"
  local asset_name
  local upload_file
  local upload_exit=0
  local upload_status=""
  asset_name="$(basename "$asset")"
  upload_file="$TMP_DIR/upload-${asset_name}.json"

  echo "uploading Gitee release asset: $asset_name"
  if curl_request "$upload_file" 1 \
    --request POST \
    --header 'Expect:' \
    --form "access_token=${GITEE_SYNC_TOKEN}" \
    --form "file=@${asset}" \
    "${RELEASES_URL}/${release_id}/attach_files"; then
    upload_status="$CURL_HTTP_STATUS"
  else
    upload_exit=$?
  fi

  # POST is attempted only once per workflow run. Whether it succeeded, timed
  # out, or lost its response, query Gitee until the remote state is clear.
  # If the asset remains absent, stop and let a later workflow run resume it;
  # never repeat an ambiguous multipart POST in the same run.
  if confirm_uploaded_asset "$asset"; then
    echo "confirmed Gitee release asset: $asset_name"
    uploaded_count=$((uploaded_count + 1))
    return
  fi

  if [[ "$upload_exit" -ne 0 ]]; then
    echo "upload response was interrupted and the asset is not visible: $asset_name (curl exit $upload_exit)" >&2
  elif is_success_http_status "$upload_status"; then
    echo "Gitee accepted the upload but the asset is not visible yet: $asset_name" >&2
  else
    echo "Gitee rejected the upload and the asset is absent: $asset_name (HTTP $upload_status)" >&2
    jq -c . "$upload_file" >&2 2>/dev/null || true
  fi
  echo "stopping safely instead of risking a duplicate attachment; rerun the workflow to resume" >&2
  exit 1
}

verify_complete_asset_set() {
  local expected_names="$TMP_DIR/expected-asset-names"
  local remote_names="$TMP_DIR/remote-asset-names"
  local remote_count
  local unique_remote_count

  {
    find "$ASSETS_DIR" -maxdepth 1 -type f ! -name checksums.txt -exec basename {} \;
    basename "$ASSETS_DIR/checksums.txt"
  } | sort > "$expected_names"
  jq -r '.[].name' "$attachments_file" | sort > "$remote_names"

  remote_count="$(jq 'length' "$attachments_file")"
  unique_remote_count="$(jq '[.[].name] | unique | length' "$attachments_file")"
  if [[ "$remote_count" -ne "$unique_remote_count" ]]; then
    echo "Gitee release contains duplicate asset names" >&2
    exit 1
  fi
  if ! cmp -s "$expected_names" "$remote_names"; then
    echo "published Gitee release asset set does not exactly match the current Gitee build" >&2
    diff -u "$expected_names" "$remote_names" >&2 || true
    exit 1
  fi
}

uploaded_count=0
skipped_count=0
while IFS= read -r -d '' asset; do
  classify_asset "$asset"
done < <(find "$ASSETS_DIR" -maxdepth 1 -type f ! -name checksums.txt -print0 | sort -z)
classify_asset "$ASSETS_DIR/checksums.txt"

while IFS= read -r -d '' asset; do
  upload_missing_asset "$asset"
done < "$missing_assets_file"

refresh_attachments
verify_complete_asset_set
echo "published $uploaded_count missing assets and kept $skipped_count identical assets for $TAG"
