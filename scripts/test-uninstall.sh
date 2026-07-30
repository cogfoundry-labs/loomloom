#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
test_root="$(mktemp -d)"
test_root="$(cd -P "$test_root" && pwd -P)"
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

failure_output=""
expect_failure() {
  local status
  set +e
  failure_output="$("$@" 2>&1)"
  status=$?
  set -e
  [[ "$status" -ne 0 ]] || fail "command unexpectedly succeeded: $*"
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
  printf '%s\n' \
    '---' \
    'name: loomloom' \
    'description: LoomLoom test Skill' \
    '---' \
    '# LoomLoom' \
    '' \
    '- [Setup](references/setup.md)' >"$skill_dir/SKILL.md"
  printf '# Setup\n' >"$skill_dir/references/setup.md"
}

write_misleading_reference_fixture() {
  local skill_dir="$1"
  write_skill_fixture "$skill_dir"
  sed 's#- \[Setup\](references/setup.md)#Plain text references/setup.md and [Different](references/setup.md.bak)#' \
    "$skill_dir/SKILL.md" >"$skill_dir/SKILL.tmp"
  mv "$skill_dir/SKILL.tmp" "$skill_dir/SKILL.md"
}

write_cli_fixture() {
  local install_dir="$1" binary_name="$2"
  mkdir -p "$install_dir"
  cp /usr/bin/true "$install_dir/$binary_name"
}

expect_shell_skill_refusal() {
  local home_dir="$1" skill_dir="$2" expected="$3"
  expect_failure env -i HOME="$home_dir" PATH="$test_path" \
    /bin/bash "$repo_root/uninstall.sh" --skill-only --skill-dir "$skill_dir"
  assert_contains "$failure_output" "$expected"
}

expect_powershell_skill_refusal() {
  local pwsh_path="$1" home_dir="$2" skill_dir="$3" expected="$4"
  expect_failure env -i HOME="$home_dir" APPDATA="$home_dir/AppData/Roaming" PATH="$test_path" \
    "$pwsh_path" -NoProfile -File "$repo_root/uninstall.ps1" -SkillOnly -SkillDir "$skill_dir"
  assert_contains "$failure_output" "$expected"
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

run_shell_safety_tests() {
  local case_root="$test_root/shell safety"
  local home_dir skill_dir install_dir config_file output agent parent_dir

  home_dir="$case_root/default/home"
  skill_dir="$home_dir/.codex/skills/loomloom"
  write_skill_fixture "$skill_dir"
  output="$(
    env -i HOME="$home_dir" PATH="$test_path" \
      /bin/bash "$repo_root/uninstall.sh" --skill-only
  )"
  [[ ! -e "$skill_dir" ]] || fail "default LoomLoom Skill was not removed"
  assert_contains "$output" "removed:"

  home_dir="$case_root/home target"
  write_skill_fixture "$home_dir"
  expect_shell_skill_refusal "$home_dir" "$home_dir/./" "dangerous path"
  [[ -e "$home_dir/SKILL.md" ]] || fail "HOME was removed"

  for agent in codex claude openclaw; do
    home_dir="$case_root/$agent parent/home"
    case "$agent" in
      codex) parent_dir="$home_dir/.codex/skills" ;;
      claude) parent_dir="$home_dir/.claude/skills" ;;
      openclaw) parent_dir="$home_dir/.openclaw/workspace/skills" ;;
    esac
    write_skill_fixture "$parent_dir"
    expect_shell_skill_refusal "$home_dir" "$parent_dir" "dangerous path"
    [[ -e "$parent_dir/SKILL.md" ]] || fail "$agent Skills parent was removed"
  done

  home_dir="$case_root/symlink/home"
  skill_dir="$case_root/symlink/real-skill"
  write_skill_fixture "$skill_dir"
  ln -s "$skill_dir" "$case_root/symlink/linked-skill"
  expect_shell_skill_refusal "$home_dir" "$case_root/symlink/linked-skill/./" "symbolic link"
  [[ -e "$skill_dir/SKILL.md" ]] || fail "symbolic-link target Skill was removed"

  home_dir="$case_root/root/home"
  mkdir -p "$home_dir" "$case_root/root/fake-bin"
  cp /usr/bin/false "$case_root/root/fake-bin/rm"
  expect_failure env -i HOME="$home_dir" PATH="$case_root/root/fake-bin:$test_path" \
    /bin/bash "$repo_root/uninstall.sh" --skill-only --skill-dir /
  assert_contains "$failure_output" "dangerous path"

  home_dir="$case_root/identity/home"
  skill_dir="$case_root/identity/skill"
  write_skill_fixture "$skill_dir"
  sed 's/^name: loomloom$/name: another-skill/' "$skill_dir/SKILL.md" >"$skill_dir/SKILL.tmp"
  mv "$skill_dir/SKILL.tmp" "$skill_dir/SKILL.md"
  expect_shell_skill_refusal "$home_dir" "$skill_dir" "frontmatter"
  [[ -e "$skill_dir" ]] || fail "non-LoomLoom Skill was removed"

  home_dir="$case_root/unrelated/home"
  skill_dir="$case_root/unrelated/skill"
  write_skill_fixture "$skill_dir"
  printf 'keep\n' >"$skill_dir/unrelated.txt"
  expect_shell_skill_refusal "$home_dir" "$skill_dir" "unexpected top-level entry"
  [[ -e "$skill_dir/unrelated.txt" ]] || fail "Skill directory with unrelated file was removed"

  home_dir="$case_root/unreferenced/home"
  skill_dir="$case_root/unreferenced/skill"
  write_skill_fixture "$skill_dir"
  printf '# Extra\n' >"$skill_dir/references/extra.md"
  expect_shell_skill_refusal "$home_dir" "$skill_dir" "reference is not explicitly"
  [[ -e "$skill_dir/references/extra.md" ]] || fail "unreferenced reference was removed"

  home_dir="$case_root/misleading reference/home"
  skill_dir="$case_root/misleading reference/skill"
  write_misleading_reference_fixture "$skill_dir"
  expect_shell_skill_refusal "$home_dir" "$skill_dir" "reference is not explicitly"
  [[ -e "$skill_dir/references/setup.md" ]] || fail "misleading reference removed the Skill"

  home_dir="$case_root/referenced/home"
  skill_dir="$case_root/referenced/parent/skill"
  write_skill_fixture "$skill_dir"
  printf '\n- [Extra](references/extra.md#details)\n' >>"$skill_dir/SKILL.md"
  printf '# Extra\n' >"$skill_dir/references/extra.md"
  output="$(
    env -i HOME="$home_dir" PATH="$test_path" \
      /bin/bash "$repo_root/uninstall.sh" --skill-only \
        --skill-dir "$skill_dir/../skill/"
  )"
  [[ ! -e "$skill_dir" ]] || fail "new explicitly referenced reference was not removed"
  assert_contains "$output" "removed:"

  home_dir="$case_root/preflight/home"
  install_dir="$case_root/preflight/bin"
  skill_dir="$case_root/preflight/skill"
  config_file="$(shell_config_file "$home_dir")"
  write_cli_fixture "$install_dir" loomloom
  write_skill_fixture "$skill_dir"
  printf 'keep\n' >"$skill_dir/unrelated.txt"
  mkdir -p "$(dirname "$config_file")"
  printf '{}\n' >"$config_file"
  expect_failure env -i HOME="$home_dir" PATH="$test_path" XDG_CONFIG_HOME="$home_dir/.config" \
    /bin/bash "$repo_root/uninstall.sh" --install-dir "$install_dir" --skill-dir "$skill_dir"
  [[ -e "$install_dir/loomloom" ]] || fail "CLI was removed before Skill validation failed"
  [[ -e "$config_file" ]] || fail "config was removed before Skill validation failed"

  home_dir="$case_root/mutual/home"
  install_dir="$case_root/mutual/bin"
  skill_dir="$case_root/mutual/skill"
  write_cli_fixture "$install_dir" loomloom
  write_skill_fixture "$skill_dir"
  expect_failure env -i HOME="$home_dir" PATH="$test_path" \
    /bin/bash "$repo_root/uninstall.sh" --cli-only --skill-only \
      --install-dir "$install_dir" --skill-dir "$skill_dir"
  assert_contains "$failure_output" "cli-only and skill-only cannot be used together"
  [[ -e "$install_dir/loomloom" && -e "$skill_dir" ]] || fail "mutually exclusive Bash flags removed files"

  expect_failure env -i HOME="$home_dir" PATH="$test_path" \
    /bin/bash "$repo_root/uninstall.sh" --skill-only --cli-only \
      --install-dir "$install_dir" --skill-dir "$skill_dir"
  assert_contains "$failure_output" "cli-only and skill-only cannot be used together"
  [[ -e "$install_dir/loomloom" && -e "$skill_dir" ]] || fail "reversed mutually exclusive Bash flags removed files"
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

run_powershell_safety_tests() {
  local pwsh_path
  pwsh_path="$(command -v pwsh || true)"
  if [[ -z "$pwsh_path" ]]; then
    echo "PowerShell unavailable; skipped uninstall.ps1 safety tests"
    return
  fi

  local case_root="$test_root/powershell safety"
  local home_dir="$case_root/home"
  local app_data="$home_dir/AppData/Roaming"
  local install_dir="$case_root/bin" skill_dir="$case_root/skill"

  write_cli_fixture "$install_dir" loomloom.exe
  write_skill_fixture "$skill_dir"
  expect_failure env -i HOME="$home_dir" APPDATA="$app_data" PATH="$test_path" \
    "$pwsh_path" -NoProfile -File "$repo_root/uninstall.ps1" \
      -CliOnly -SkillOnly -InstallDir "$install_dir" -SkillDir "$skill_dir"
  assert_contains "$failure_output" "cli-only and skill-only cannot be used together"
  [[ -e "$install_dir/loomloom.exe" && -e "$skill_dir" ]] || fail "mutually exclusive PowerShell flags removed files"

  local dangerous_home="$case_root/dangerous home"
  write_skill_fixture "$dangerous_home"
  expect_powershell_skill_refusal "$pwsh_path" "$dangerous_home" "$dangerous_home/./" "dangerous path"
  [[ -e "$dangerous_home/SKILL.md" ]] || fail "PowerShell removed HOME"

  local agent parent_home parent_dir
  for agent in codex claude openclaw; do
    parent_home="$case_root/$agent parent/home"
    case "$agent" in
      codex) parent_dir="$parent_home/.codex/skills" ;;
      claude) parent_dir="$parent_home/.claude/skills" ;;
      openclaw) parent_dir="$parent_home/.openclaw/workspace/skills" ;;
    esac
    write_skill_fixture "$parent_dir"
    expect_powershell_skill_refusal "$pwsh_path" "$parent_home" "$parent_dir" "dangerous path"
    [[ -e "$parent_dir/SKILL.md" ]] || fail "PowerShell removed $agent Skills parent"
  done

  local real_skill="$case_root/symlink/real-skill"
  local linked_skill="$case_root/symlink/linked-skill"
  write_skill_fixture "$real_skill"
  ln -s "$real_skill" "$linked_skill"
  expect_powershell_skill_refusal "$pwsh_path" "$home_dir" "$linked_skill/./" "symbolic link or"
  [[ -e "$real_skill/SKILL.md" ]] || fail "PowerShell removed symbolic-link target Skill"

  local root_home="$case_root/root home"
  mkdir -p "$root_home"
  # shellcheck disable=SC2016 # PowerShell expands these environment variables.
  expect_failure env -i \
    HOME="$root_home" \
    APPDATA="$root_home/AppData/Roaming" \
    PATH="$test_path" \
    UNINSTALL_TEST_SCRIPT="$repo_root/uninstall.ps1" \
    UNINSTALL_TEST_ROOT="/" \
    "$pwsh_path" -NoProfile -Command \
      'function Remove-Item { throw "Remove-Item must not be called" }; & $env:UNINSTALL_TEST_SCRIPT -SkillOnly -SkillDir $env:UNINSTALL_TEST_ROOT'
  assert_contains "$failure_output" "dangerous path"
  assert_not_contains "$failure_output" "Remove-Item must not be called"

  local invalid_dir="$case_root/invalid skill"
  write_skill_fixture "$invalid_dir"
  sed 's/^name: loomloom$/name: another-skill/' "$invalid_dir/SKILL.md" >"$invalid_dir/SKILL.tmp"
  mv "$invalid_dir/SKILL.tmp" "$invalid_dir/SKILL.md"
  expect_powershell_skill_refusal "$pwsh_path" "$home_dir" "$invalid_dir" "frontmatter"
  [[ -e "$invalid_dir" ]] || fail "PowerShell removed non-LoomLoom Skill"

  local preflight_home="$case_root/preflight/home"
  local preflight_app_data="$preflight_home/AppData/Roaming"
  local preflight_install_dir="$case_root/preflight/bin"
  local preflight_skill_dir="$case_root/preflight/skill"
  local preflight_config_file="$preflight_app_data/loomloom/config.json"
  write_cli_fixture "$preflight_install_dir" loomloom.exe
  write_skill_fixture "$preflight_skill_dir"
  printf 'keep\n' >"$preflight_skill_dir/unrelated.txt"
  mkdir -p "$(dirname "$preflight_config_file")"
  printf '{}\n' >"$preflight_config_file"
  expect_failure env -i HOME="$preflight_home" APPDATA="$preflight_app_data" PATH="$test_path" \
    "$pwsh_path" -NoProfile -File "$repo_root/uninstall.ps1" \
      -InstallDir "$preflight_install_dir" -SkillDir "$preflight_skill_dir"
  [[ -e "$preflight_install_dir/loomloom.exe" ]] || fail "PowerShell removed CLI before Skill validation failed"
  [[ -e "$preflight_config_file" ]] || fail "PowerShell removed config before Skill validation failed"

  local unrelated_dir="$case_root/unrelated skill"
  write_skill_fixture "$unrelated_dir"
  printf 'keep\n' >"$unrelated_dir/unrelated.txt"
  expect_powershell_skill_refusal "$pwsh_path" "$home_dir" "$unrelated_dir" "unexpected top-level entry"
  [[ -e "$unrelated_dir/unrelated.txt" ]] || fail "PowerShell removed unrelated Skill file"

  local unreferenced_dir="$case_root/unreferenced skill"
  write_skill_fixture "$unreferenced_dir"
  printf '# Extra\n' >"$unreferenced_dir/references/extra.md"
  expect_powershell_skill_refusal "$pwsh_path" "$home_dir" "$unreferenced_dir" "reference is not explicitly"
  [[ -e "$unreferenced_dir/references/extra.md" ]] || fail "PowerShell removed unreferenced reference"

  local misleading_dir="$case_root/misleading reference skill"
  write_misleading_reference_fixture "$misleading_dir"
  expect_powershell_skill_refusal "$pwsh_path" "$home_dir" "$misleading_dir" "reference is not explicitly"
  [[ -e "$misleading_dir/references/setup.md" ]] || fail "PowerShell removed misleading reference Skill"

  local referenced_dir="$case_root/referenced skill"
  write_skill_fixture "$referenced_dir"
  printf '\n- [Extra](references/extra.md#details)\n' >>"$referenced_dir/SKILL.md"
  printf '# Extra\n' >"$referenced_dir/references/extra.md"
  env -i HOME="$home_dir" APPDATA="$app_data" PATH="$test_path" \
    "$pwsh_path" -NoProfile -File "$repo_root/uninstall.ps1" -SkillOnly -SkillDir "$referenced_dir" >/dev/null
  [[ ! -e "$referenced_dir" ]] || fail "PowerShell did not remove explicitly referenced reference"

  local brew_bin="$case_root/brew-bin"
  mkdir -p "$brew_bin"
  # shellcheck disable=SC2016 # The generated fake brew expands its own arguments.
  printf '%s\n' \
    '#!/usr/bin/env bash' \
    'if [[ "${1:-}" == "list" ]]; then exit 0; fi' \
    'if [[ "${1:-}" == "uninstall" ]]; then exit 23; fi' \
    'exit 1' >"$brew_bin/brew"
  chmod +x "$brew_bin/brew"
  expect_failure env -i HOME="$home_dir" APPDATA="$app_data" PATH="$brew_bin:$test_path" \
    "$pwsh_path" -NoProfile -File "$repo_root/uninstall.ps1" -CliOnly -InstallDir "$case_root/missing-bin"
  assert_contains "$failure_output" "failed to uninstall Homebrew formula: loomloom"
  assert_not_contains "$failure_output" "removed Homebrew formula: loomloom"
}

run_shell_full_uninstall_test
run_shell_legacy_config_test
run_shell_partial_uninstall_tests
run_shell_safety_tests
run_powershell_full_uninstall_test
run_powershell_safety_tests

echo "uninstall script tests passed"
