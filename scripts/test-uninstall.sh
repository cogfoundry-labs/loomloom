#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
test_root="$(mktemp -d)"
test_path="/usr/bin:/bin"
trap 'rm -rf "$test_root"' EXIT

fail() {
  echo "uninstall test failed: $*" >&2
  exit 1
}

assert_contains() {
  local text="$1" expected="$2"
  [[ "$text" == *"$expected"* ]] || fail "output missing: $expected"
}

assert_not_contains() {
  local text="$1" unexpected="$2"
  [[ "$text" != *"$unexpected"* ]] || fail "output unexpectedly contains: $unexpected"
}

shell_config_file() {
  local home_dir="$1"
  case "$(uname -s)" in
    Darwin) printf '%s\n' "$home_dir/Library/Application Support/loomloom/config.json" ;;
    *) printf '%s\n' "$home_dir/.config/loomloom/config.json" ;;
  esac
}

write_skill_fixture() {
  local skill_dir="$1"
  mkdir -p "$skill_dir/references"
  printf '# LoomLoom\n' >"$skill_dir/SKILL.md"
  printf '# Setup\n' >"$skill_dir/references/setup.md"
}

write_cli_fixture() {
  local install_dir="$1" binary_name="$2"
  mkdir -p "$install_dir"
  cp /usr/bin/true "$install_dir/$binary_name"
}

run_shell_full_uninstall_test() {
  local case_root="$test_root/shell full"
  local home_dir="$case_root/home"
  local install_dir="$case_root/bin" skill_dir="$case_root/skill pack"
  local config_file output count
  config_file="$(shell_config_file "$home_dir")"
  mkdir -p "$(dirname "$config_file")"
  write_cli_fixture "$install_dir" loomloom
  write_skill_fixture "$skill_dir"
  printf '%s\n' \
    '{' \
    '  "active_server": "shengsuanyun",' \
    '  "servers": [' \
    '    {' \
    '      "name": "shengsuanyun",' \
    '      "token_env": "LOOMLOOM_TOKEN_SHENGSUANYUN",' \
    '      "token": "browser-secret-must-not-appear"' \
    '    },' \
    '    {' \
    '      "name": "active",' \
    '      "token_env": "LOOMLOOM_TOKEN_ACTIVE"' \
    '    },' \
    '    {' \
    '      "name": "invalid",' \
    '      "token_env": "NOT_ALLOWED"' \
    '    }' \
    '  ]' \
    '}' >"$config_file"

  output="$(
    env -i \
      HOME="$home_dir" \
      PATH="$test_path" \
      XDG_CONFIG_HOME="$home_dir/.config" \
      LOOMLOOM_TOKEN_ACTIVE="environment-secret-must-not-appear" \
      LOOMLOOM_SERVER="server-must-not-be-reported" \
      /bin/bash "$repo_root/uninstall.sh" \
        --install-dir "$install_dir" \
        --skill-dir "$skill_dir"
  )"

  [[ ! -e "$install_dir/loomloom" ]] || fail "shell CLI was not removed"
  [[ ! -e "$skill_dir" ]] || fail "shell Skill directory was not removed"
  [[ ! -e "$config_file" ]] || fail "shell config.json was not removed"
  [[ ! -e "$(dirname "$config_file")" ]] || fail "empty shell config directory was not removed"
  assert_contains "$output" "environment token cleanup required: LOOMLOOM_TOKEN_SHENGSUANYUN"
  assert_contains "$output" "environment token cleanup required: LOOMLOOM_TOKEN_ACTIVE"
  assert_contains "$output" "Agent action required: ask the user for confirmation"
  assert_not_contains "$output" "NOT_ALLOWED"
  assert_not_contains "$output" "LOOMLOOM_SERVER"
  assert_not_contains "$output" "browser-secret-must-not-appear"
  assert_not_contains "$output" "environment-secret-must-not-appear"
  assert_not_contains "$output" "server-must-not-be-reported"
  count="$(grep -c 'environment token cleanup required: LOOMLOOM_TOKEN_ACTIVE' <<<"$output")"
  [[ "$count" -eq 1 ]] || fail "duplicate environment Token name was reported"

  output="$(
    env -i \
      HOME="$home_dir" \
      PATH="$test_path" \
      XDG_CONFIG_HOME="$home_dir/.config" \
      /bin/bash "$repo_root/uninstall.sh" \
        --install-dir "$install_dir" \
        --skill-dir "$skill_dir"
  )"
  assert_contains "$output" "nothing removed"
}

run_shell_legacy_config_test() {
  local case_root="$test_root/shell legacy"
  local home_dir="$case_root/home"
  local install_dir="$case_root/bin" skill_dir="$case_root/skill"
  local config_file output
  config_file="$(shell_config_file "$home_dir")"
  mkdir -p "$(dirname "$config_file")"
  write_cli_fixture "$install_dir" loomloom
  write_skill_fixture "$skill_dir"
  printf '%s\n' \
    '{' \
    '  "platform": "shengsuanyun",' \
    '  "server": "https://example.invalid/loom/v1",' \
    '  "token": "legacy-config-secret-must-not-appear"' \
    '}' >"$config_file"
  printf 'keep\n' >"$(dirname "$config_file")/keep.txt"

  output="$(
    env -i \
      HOME="$home_dir" \
      PATH="$test_path" \
      XDG_CONFIG_HOME="$home_dir/.config" \
      LOOMLOOM_TOKEN="legacy-environment-secret-must-not-appear" \
      /bin/bash "$repo_root/uninstall.sh" \
        --install-dir "$install_dir" \
        --skill-dir "$skill_dir"
  )"

  [[ ! -e "$config_file" ]] || fail "legacy config.json was not removed"
  [[ -e "$(dirname "$config_file")/keep.txt" ]] || fail "unrelated config file was removed"
  assert_contains "$output" "environment token cleanup required: LOOMLOOM_TOKEN"
  assert_not_contains "$output" "legacy-config-secret-must-not-appear"
  assert_not_contains "$output" "legacy-environment-secret-must-not-appear"
}

run_shell_partial_uninstall_tests() {
  local case_root="$test_root/shell partial"
  local home_dir="$case_root/home"
  local install_dir="$case_root/bin" skill_dir="$case_root/skill"
  local config_file output
  config_file="$(shell_config_file "$home_dir")"
  mkdir -p "$(dirname "$config_file")"
  write_cli_fixture "$install_dir" loomloom
  write_skill_fixture "$skill_dir"
  printf '%s\n' '{"servers":[{"token_env":"LOOMLOOM_TOKEN_PARTIAL"}]}' >"$config_file"

  output="$(
    env -i HOME="$home_dir" PATH="$test_path" XDG_CONFIG_HOME="$home_dir/.config" \
      /bin/bash "$repo_root/uninstall.sh" --cli-only --install-dir "$install_dir"
  )"
  [[ ! -e "$install_dir/loomloom" ]] || fail "--cli-only did not remove the CLI"
  [[ -e "$skill_dir/references/setup.md" ]] || fail "--cli-only removed the Skill"
  [[ -e "$config_file" ]] || fail "--cli-only removed config.json"
  assert_not_contains "$output" "environment token cleanup required"

  write_cli_fixture "$install_dir" loomloom
  output="$(
    env -i HOME="$home_dir" PATH="$test_path" XDG_CONFIG_HOME="$home_dir/.config" \
      /bin/bash "$repo_root/uninstall.sh" --skill-only --skill-dir "$skill_dir"
  )"
  [[ ! -e "$skill_dir" ]] || fail "--skill-only did not remove the complete Skill"
  [[ -e "$install_dir/loomloom" ]] || fail "--skill-only removed the CLI"
  [[ -e "$config_file" ]] || fail "--skill-only removed config.json"
  assert_not_contains "$output" "environment token cleanup required"
}

run_powershell_full_uninstall_test() {
  local pwsh_path
  pwsh_path="$(command -v pwsh || true)"
  if [[ -z "$pwsh_path" ]]; then
    echo "PowerShell unavailable; skipped uninstall.ps1 behavior test"
    return
  fi

  local case_root="$test_root/powershell full"
  local home_dir="$case_root/home"
  local app_data="$home_dir/AppData/Roaming" install_dir="$case_root/bin"
  local skill_dir="$case_root/skill pack" config_file="$app_data/loomloom/config.json"
  local output
  mkdir -p "$(dirname "$config_file")"
  write_cli_fixture "$install_dir" loomloom.exe
  write_skill_fixture "$skill_dir"
  printf '%s\n' \
    '{' \
    '  "servers": [' \
    '    {"token_env":"LOOMLOOM_TOKEN_POWERSHELL","token":"ps-secret-must-not-appear"}' \
    '  ]' \
    '}' >"$config_file"

  output="$(
    env -i \
      HOME="$home_dir" \
      APPDATA="$app_data" \
      PATH="$test_path" \
      LOOMLOOM_TOKEN_ACTIVE_PS="ps-environment-secret-must-not-appear" \
      LOOMLOOM_SERVER="server-must-not-be-reported" \
      "$pwsh_path" -NoProfile -File "$repo_root/uninstall.ps1" \
        -InstallDir "$install_dir" \
        -SkillDir "$skill_dir"
  )"

  [[ ! -e "$install_dir/loomloom.exe" ]] || fail "PowerShell CLI was not removed"
  [[ ! -e "$skill_dir" ]] || fail "PowerShell Skill directory was not removed"
  [[ ! -e "$config_file" ]] || fail "PowerShell config.json was not removed"
  [[ ! -e "$(dirname "$config_file")" ]] || fail "empty PowerShell config directory was not removed"
  assert_contains "$output" "environment token cleanup required: LOOMLOOM_TOKEN_POWERSHELL"
  assert_contains "$output" "environment token cleanup required: LOOMLOOM_TOKEN_ACTIVE_PS"
  assert_contains "$output" "Agent action required: ask the user for confirmation"
  assert_not_contains "$output" "LOOMLOOM_SERVER"
  assert_not_contains "$output" "ps-secret-must-not-appear"
  assert_not_contains "$output" "ps-environment-secret-must-not-appear"
  assert_not_contains "$output" "server-must-not-be-reported"
}

run_shell_full_uninstall_test
run_shell_legacy_config_test
run_shell_partial_uninstall_tests
run_powershell_full_uninstall_test

echo "uninstall script tests passed"
