#!/usr/bin/env bash
set -euo pipefail

GITEE_INSTALL_URL="${GITEE_INSTALL_URL:-https://gitee.com/cogfoundry/loomloom/raw/main/install.sh}"

usage() {
  cat <<'EOF'
Usage: install-gitee.sh [install.sh options]

Install LoomLoom from the CogFoundry Gitee mirror.

This wrapper forces:
  --source gitee

Examples:
  curl -fsSL https://gitee.com/cogfoundry/loomloom/raw/main/install-gitee.sh | bash
  curl -fsSL https://gitee.com/cogfoundry/loomloom/raw/main/install-gitee.sh | bash -s -- --version v0.1.11

EOF
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

run_install() {
  local install_script="$1"
  shift
  LOOMLOOM_RELEASE_SOURCE=gitee bash "$install_script" "$@" --source gitee
}

script_source="${BASH_SOURCE[0]:-}"
if [[ -n "$script_source" && -f "$script_source" ]]; then
  script_dir="$(cd "$(dirname "$script_source")" && pwd)"
  local_install="$script_dir/install.sh"
  if [[ -f "$local_install" ]]; then
    run_install "$local_install" "$@"
    exit 0
  fi
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

curl -fsSL -o "$tmp_dir/install.sh" "$GITEE_INSTALL_URL"
run_install "$tmp_dir/install.sh" "$@"
