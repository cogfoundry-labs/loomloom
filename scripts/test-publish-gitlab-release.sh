#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$repo_root/scripts/publish-gitlab-release.sh"

fail_test() {
  echo "test failed: $*" >&2
  exit 1
}

assert_equal() {
  local expected="$1"
  local actual="$2"
  local message="$3"
  [[ "$expected" == "$actual" ]] || \
    fail_test "$message: expected '$expected', got '$actual'"
}

CI_API_V4_URL="https://gitlab.example/api/v4"
CI_PROJECT_ID="123"
CI_PROJECT_URL="https://gitlab.example/cogfoundry/loomloom"
CI_COMMIT_TAG="v0.2.1"
CI_JOB_TOKEN="test-job-token"
GITLAB_RELEASE_PACKAGE_NAME="loomloom"
PACKAGE_NAME="$GITLAB_RELEASE_PACKAGE_NAME"

validate_release_tag "v0.2.1"
validate_release_tag "v0.2.1-internal.1"
validate_release_tag "v0.2.1-beta.2"
validate_release_tag "v0.2.1-rc.3"

assert_equal \
  "LoomLoom v0.2.1" \
  "$(release_name "v0.2.1")" \
  "stable release name"
assert_equal \
  "LoomLoom v0.2.1-internal.1 (Pre-release)" \
  "$(release_name "v0.2.1-internal.1")" \
  "prerelease name"
assert_equal \
  "https://gitlab.example/api/v4/projects/123/packages/generic/loomloom/v0.2.1/loomloom-darwin-arm64.tar.gz" \
  "$(package_file_url "loomloom-darwin-arm64.tar.gz")" \
  "package URL"
assert_equal \
  "https://gitlab.example/cogfoundry/loomloom/-/releases/v0.2.1/downloads/loomloom-darwin-arm64.tar.gz" \
  "$(release_direct_url "loomloom-darwin-arm64.tar.gz")" \
  "release direct URL"

if (validate_release_tag "v0.2" >/dev/null 2>&1); then
  fail_test "invalid tag was accepted"
fi
if (validate_release_tag "v0.2.1-preview.1" >/dev/null 2>&1); then
  fail_test "unsupported prerelease channel was accepted"
fi

test_dir="$(mktemp -d)"
trap 'rm -rf "$test_dir"' EXIT
assets_dir="$test_dir/assets"
fake_package_dir="$test_dir/package"
fake_release_file="$test_dir/release.json"
fake_links_file="$test_dir/links.json"
mkdir -p "$assets_dir" "$fake_package_dir"
printf 'test release asset\n' > "$assets_dir/loomloom-test.tar.gz"
write_test_checksums() (
  cd "$assets_dir"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum loomloom-test.tar.gz > checksums.txt
  else
    shasum -a 256 loomloom-test.tar.gz > checksums.txt
  fi
)
write_test_checksums
printf '[]\n' > "$fake_links_file"

curl() {
  local method="GET"
  local output=""
  local upload_file_path=""
  local payload=""
  local write_out="false"
  local fail_http="false"
  local url=""
  local status=""
  local filename=""
  local direct_url=""
  local response=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --request)
        method="$2"
        shift 2
        ;;
      --output)
        output="$2"
        shift 2
        ;;
      --upload-file)
        upload_file_path="$2"
        shift 2
        ;;
      --data)
        payload="$2"
        shift 2
        ;;
      --write-out)
        write_out="true"
        shift 2
        ;;
      --header)
        shift 2
        ;;
      --fail)
        fail_http="true"
        shift
        ;;
      --silent|--show-error|--location)
        shift
        ;;
      http://*|https://*)
        url="$1"
        shift
        ;;
      *) fail_test "fake curl received unsupported argument: $1" ;;
    esac
  done

  if [[ "$url" == *"/packages/generic/"* ]]; then
    filename="${url##*/}"
    if [[ "$method" == "PUT" ]]; then
      cp "$upload_file_path" "$fake_package_dir/$filename"
      status="201"
      response='{}'
    elif [[ -f "$fake_package_dir/$filename" ]]; then
      cp "$fake_package_dir/$filename" "$output"
      status="200"
    else
      status="404"
      response='{}'
    fi
  elif [[ "$url" == *"/releases/"*"/assets/links" ]]; then
    if [[ "$method" == "POST" ]]; then
      direct_url="${CI_PROJECT_URL}/-/releases/${CI_COMMIT_TAG}/downloads/$(jq -r '.name' <<<"$payload")"
      response="$(jq --arg direct_url "$direct_url" '. + {id: 1, direct_asset_url: $direct_url}' <<<"$payload")"
      jq --argjson link "$response" '. + [$link]' "$fake_links_file" > "$fake_links_file.next"
      mv "$fake_links_file.next" "$fake_links_file"
      status="201"
    else
      response="$(cat "$fake_links_file")"
      status="200"
    fi
  elif [[ "$url" == *"/releases/"*"/downloads/"* ]]; then
    filename="${url##*/}"
    if [[ -f "$fake_package_dir/$filename" ]]; then
      cp "$fake_package_dir/$filename" "$output"
      status="200"
    else
      status="404"
    fi
  elif [[ "$url" == *"/releases/"* ]]; then
    if [[ -f "$fake_release_file" ]]; then
      response="$(cat "$fake_release_file")"
      status="200"
    else
      status="404"
      response='{}'
    fi
  elif [[ "$url" == */releases && "$method" == "POST" ]]; then
    response="$(jq '. + {created_at: "2026-08-02T00:00:00Z"}' <<<"$payload")"
    printf '%s\n' "$response" > "$fake_release_file"
    status="201"
  else
    fail_test "fake curl received unsupported URL: $url"
  fi

  if [[ -n "$output" && -n "$response" ]]; then
    printf '%s\n' "$response" > "$output"
  fi
  if [[ "$fail_http" == "true" && ! "$status" =~ ^2 ]]; then
    return 22
  fi
  if [[ "$write_out" == "true" ]]; then
    printf '%s' "$status"
  fi
}

upload_package "$assets_dir"
create_release "$assets_dir"
upload_package "$assets_dir"
create_release "$assets_dir"

assert_equal "2" "$(find "$fake_package_dir" -type f | wc -l | tr -d ' ')" "published package asset count"
assert_equal "2" "$(jq 'length' "$fake_links_file")" "release asset link count"

printf 'changed release asset\n' > "$assets_dir/loomloom-test.tar.gz"
write_test_checksums
if (upload_package "$assets_dir" >/dev/null 2>&1); then
  fail_test "a published package asset was overwritten with different content"
fi

echo "GitLab Release publishing tests passed"
