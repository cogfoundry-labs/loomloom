#!/usr/bin/env bash
set -euo pipefail

PACKAGE_NAME="${GITLAB_RELEASE_PACKAGE_NAME:-loomloom}"

usage() {
  cat <<'EOF'
Usage: scripts/publish-gitlab-release.sh <upload|release> [assets-dir]

Publish verified release assets to the GitLab Generic Package Registry, then
create or verify the GitLab Release and its permanent asset links.

Required environment:
  CI_API_V4_URL
  CI_PROJECT_ID
  CI_PROJECT_URL
  CI_JOB_TOKEN
  CI_COMMIT_TAG
EOF
}

fail() {
  echo "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

require_env() {
  local name="$1"
  [[ -n "${!name:-}" ]] || fail "required environment variable is unset: $name"
}

validate_release_tag() {
  local tag="$1"
  [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-(internal|beta|rc)\.[0-9]+)?$ ]] || \
    fail "unsupported release tag: $tag"
}

is_prerelease_tag() {
  [[ "$1" =~ -(internal|beta|rc)\.[0-9]+$ ]]
}

release_name() {
  local tag="$1"
  if is_prerelease_tag "$tag"; then
    printf 'LoomLoom %s (Pre-release)\n' "$tag"
  else
    printf 'LoomLoom %s\n' "$tag"
  fi
}

release_description() {
  local tag="$1"
  if is_prerelease_tag "$tag"; then
    # shellcheck disable=SC2016
    printf 'Pre-release build for `%s`.\n\nThe attached assets were built and verified by GitLab CI.\n' "$tag"
  else
    # shellcheck disable=SC2016
    printf 'Release `%s`.\n\nThe attached assets were built and verified by GitLab CI.\n' "$tag"
  fi
}

url_encode() {
  jq -nr --arg value "$1" '$value|@uri'
}

package_base_url() {
  printf '%s/projects/%s/packages/generic/%s/%s\n' \
    "$CI_API_V4_URL" "$CI_PROJECT_ID" "$PACKAGE_NAME" "$CI_COMMIT_TAG"
}

package_file_url() {
  local filename="$1"
  printf '%s/%s\n' "$(package_base_url)" "$(url_encode "$filename")"
}

release_direct_url() {
  local filename="$1"
  printf '%s/-/releases/%s/downloads/%s\n' \
    "$CI_PROJECT_URL" "$(url_encode "$CI_COMMIT_TAG")" "$(url_encode "$filename")"
}

list_assets() {
  local assets_dir="$1"
  find "$assets_dir" -maxdepth 1 -type f -print | LC_ALL=C sort
}

validate_environment() {
  require_env CI_API_V4_URL
  require_env CI_PROJECT_ID
  require_env CI_PROJECT_URL
  require_env CI_JOB_TOKEN
  require_env CI_COMMIT_TAG
  validate_release_tag "$CI_COMMIT_TAG"
}

validate_assets() {
  local assets_dir="$1"
  local repo_root
  repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
  "$repo_root/scripts/validate-release-assets.sh" "$assets_dir"
}

download_with_status() {
  local url="$1"
  local output="$2"
  curl \
    --silent \
    --show-error \
    --location \
    --header "JOB-TOKEN: ${CI_JOB_TOKEN}" \
    --output "$output" \
    --write-out '%{http_code}' \
    "$url"
}

upload_file() (
  local file="$1"
  local filename
  local url
  local existing
  local response
  local status

  filename="$(basename "$file")"
  url="$(package_file_url "$filename")"
  existing="$(mktemp)"
  response="$(mktemp)"
  trap 'rm -f "$existing" "$response"' EXIT

  status="$(download_with_status "$url" "$existing")"
  case "$status" in
    200)
      if cmp -s "$file" "$existing"; then
        echo "GitLab package asset already matches: $filename"
        return
      fi
      fail "GitLab package asset already exists with different content: $filename"
      ;;
    404) ;;
    *) fail "failed to inspect GitLab package asset $filename: HTTP $status" ;;
  esac

  status="$(
    curl \
      --silent \
      --show-error \
      --location \
      --request PUT \
      --header "JOB-TOKEN: ${CI_JOB_TOKEN}" \
      --upload-file "$file" \
      --output "$response" \
      --write-out '%{http_code}' \
      "$url"
  )"
  [[ "$status" == "201" ]] || \
    fail "failed to upload GitLab package asset $filename: HTTP $status: $(cat "$response")"
  echo "uploaded GitLab package asset: $filename"
)

upload_package() (
  local assets_dir="$1"
  local verification_dir
  local file
  local filename
  local status

  require_cmd curl
  require_cmd jq
  validate_assets "$assets_dir"

  while IFS= read -r file; do
    upload_file "$file"
  done < <(list_assets "$assets_dir")

  verification_dir="$(mktemp -d)"
  trap 'rm -rf "$verification_dir"' EXIT
  while IFS= read -r file; do
    filename="$(basename "$file")"
    status="$(download_with_status "$(package_file_url "$filename")" "$verification_dir/$filename")"
    [[ "$status" == "200" ]] || \
      fail "failed to download GitLab package asset $filename: HTTP $status"
  done < <(list_assets "$assets_dir")

  cmp -s "$assets_dir/checksums.txt" "$verification_dir/checksums.txt" || \
    fail "published checksums.txt does not match the current build"
  validate_assets "$verification_dir"
  echo "verified GitLab package: $PACKAGE_NAME/$CI_COMMIT_TAG"
)

api_get() {
  local url="$1"
  local output="$2"
  curl \
    --silent \
    --show-error \
    --location \
    --header "JOB-TOKEN: ${CI_JOB_TOKEN}" \
    --output "$output" \
    --write-out '%{http_code}' \
    "$url"
}

api_post_json() {
  local url="$1"
  local payload="$2"
  local output="$3"
  curl \
    --silent \
    --show-error \
    --location \
    --request POST \
    --header "JOB-TOKEN: ${CI_JOB_TOKEN}" \
    --header 'Content-Type: application/json' \
    --data "$payload" \
    --output "$output" \
    --write-out '%{http_code}' \
    "$url"
}

ensure_release_record() (
  local release_url="$1"
  local response
  local payload
  local status

  response="$(mktemp)"
  trap 'rm -f "$response"' EXIT
  status="$(api_get "$release_url" "$response")"
  case "$status" in
    200)
      echo "GitLab Release already exists: $CI_COMMIT_TAG"
      return
      ;;
    404) ;;
    *) fail "failed to inspect GitLab Release: HTTP $status: $(cat "$response")" ;;
  esac

  payload="$(
    jq -n \
      --arg name "$(release_name "$CI_COMMIT_TAG")" \
      --arg tag_name "$CI_COMMIT_TAG" \
      --arg description "$(release_description "$CI_COMMIT_TAG")" \
      '{name: $name, tag_name: $tag_name, description: $description}'
  )"
  status="$(api_post_json "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/releases" "$payload" "$response")"
  [[ "$status" == "201" ]] || \
    fail "failed to create GitLab Release: HTTP $status: $(cat "$response")"
  echo "created GitLab Release: $CI_COMMIT_TAG"
)

ensure_release_link() (
  local links_url="$1"
  local links_json="$2"
  local filename="$3"
  local package_url
  local direct_path
  local direct_url
  local matches
  local payload
  local response
  local status

  package_url="$(package_file_url "$filename")"
  direct_path="/$filename"
  direct_url="$(release_direct_url "$filename")"
  matches="$(jq --arg name "$filename" '[.[] | select(.name == $name)] | length' "$links_json")"
  case "$matches" in
    0) ;;
    1)
      if jq -e \
        --arg name "$filename" \
        --arg url "$package_url" \
        --arg direct_url "$direct_url" \
        '.[] | select(.name == $name) | select(.url == $url and .direct_asset_url == $direct_url)' \
        "$links_json" >/dev/null; then
        echo "GitLab Release asset link already matches: $filename"
        return
      fi
      fail "GitLab Release asset link already exists with different content: $filename"
      ;;
    *) fail "multiple GitLab Release asset links use the same name: $filename" ;;
  esac

  payload="$(
    jq -n \
      --arg name "$filename" \
      --arg url "$package_url" \
      --arg direct_asset_path "$direct_path" \
      '{name: $name, url: $url, direct_asset_path: $direct_asset_path, link_type: "package"}'
  )"
  response="$(mktemp)"
  trap 'rm -f "$response"' EXIT
  status="$(api_post_json "$links_url" "$payload" "$response")"
  [[ "$status" == "201" ]] || \
    fail "failed to create GitLab Release asset link $filename: HTTP $status: $(cat "$response")"
  echo "created GitLab Release asset link: $filename"
)

verify_public_release_assets() (
  local assets_dir="$1"
  local verification_dir
  local file
  local filename

  verification_dir="$(mktemp -d)"
  trap 'rm -rf "$verification_dir"' EXIT
  while IFS= read -r file; do
    filename="$(basename "$file")"
    curl \
      --silent \
      --show-error \
      --fail \
      --location \
      --output "$verification_dir/$filename" \
      "$(release_direct_url "$filename")"
    cmp -s "$file" "$verification_dir/$filename" || \
      fail "public GitLab Release asset does not match the current build: $filename"
  done < <(list_assets "$assets_dir")
  validate_assets "$verification_dir"
  echo "verified public GitLab Release assets: $CI_COMMIT_TAG"
)

create_release() (
  local assets_dir="$1"
  local encoded_tag
  local release_url
  local links_url
  local links_json
  local file
  local status

  require_cmd curl
  require_cmd jq
  validate_assets "$assets_dir"
  encoded_tag="$(url_encode "$CI_COMMIT_TAG")"
  release_url="${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/releases/${encoded_tag}"
  links_url="${release_url}/assets/links"
  links_json="$(mktemp)"
  trap 'rm -f "$links_json"' EXIT

  ensure_release_record "$release_url"
  status="$(api_get "$links_url" "$links_json")"
  [[ "$status" == "200" ]] || \
    fail "failed to list GitLab Release asset links: HTTP $status: $(cat "$links_json")"

  while IFS= read -r file; do
    ensure_release_link "$links_url" "$links_json" "$(basename "$file")"
  done < <(list_assets "$assets_dir")

  verify_public_release_assets "$assets_dir"
)

main() {
  local command="${1:-}"
  local assets_dir="${2:-release}"

  case "$command" in
    --help|-h)
      usage
      return
      ;;
    upload|release) ;;
    *)
      usage >&2
      exit 1
      ;;
  esac

  [[ -d "$assets_dir" ]] || fail "assets directory does not exist: $assets_dir"
  validate_environment
  case "$command" in
    upload) upload_package "$assets_dir" ;;
    release) create_release "$assets_dir" ;;
  esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
