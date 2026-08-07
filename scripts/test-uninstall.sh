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
  mkdir -p \
    "$skill_dir/references" \
    "$skill_dir/generated-template-spec/en" \
    "$skill_dir/generated-template-spec/machine"
  printf '%s\n' \
    '---' \
    'name: loomloom' \
    'description: LoomLoom test Skill' \
    '---' \
    '# LoomLoom' \
    '' \
    '- [Setup](references/setup.md)' >"$skill_dir/SKILL.md"
  printf '# Setup\n' >"$skill_dir/references/setup.md"
  printf '%s\n' \
    '{' \
    '  "owner": "loomloom-docs",' \
    '  "generator": "loomloom-template-docs/v2"' \
    '}' >"$skill_dir/generated-template-spec/manifest.json"
  printf '# TemplateSpec\n' >"$skill_dir/generated-template-spec/en/README.md"
  printf '{}\n' >"$skill_dir/generated-template-spec/machine/template-spec.schema.json"
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
  local install_dir="$case_root/bin" skill_dir="$case_root/skill pack/loomloom"
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
  local install_dir="$case_root/bin" skill_dir="$case_root/skill/loomloom"
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
  local install_dir="$case_root/bin" skill_dir="$case_root/skill/loomloom"
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
  local home_dir skill_dir install_dir config_file output parent_dir external_file

  home_dir="$case_root/missing target/home"
  skill_dir="$home_dir/custom-runtime/skills/loomloom"
  write_skill_fixture "$skill_dir"
  expect_failure env -i HOME="$home_dir" PATH="$test_path" \
    /bin/bash "$repo_root/uninstall.sh" --skill-only
  assert_contains "$failure_output" "--skill-dir is required"
  [[ -e "$skill_dir/SKILL.md" ]] || fail "missing --skill-dir removed the LoomLoom Skill"

  output="$(
    env -i HOME="$home_dir" PATH="$test_path" \
      /bin/bash "$repo_root/uninstall.sh" --skill-only --skill-dir "$skill_dir"
  )"
  [[ ! -e "$skill_dir" ]] || fail "explicit LoomLoom Skill was not removed"
  assert_contains "$output" "removed:"

  home_dir="$case_root/home target"
  write_skill_fixture "$home_dir"
  expect_shell_skill_refusal "$home_dir" "$home_dir/./" "dangerous path"
  [[ -e "$home_dir/SKILL.md" ]] || fail "HOME was removed"

  home_dir="$case_root/generic parent/home"
  for parent_dir in \
    "$case_root/custom-runtime/skills" \
    "$case_root/unknown-platform/extensions" \
    "$case_root/agent with spaces/skills"; do
    write_skill_fixture "$parent_dir"
    expect_shell_skill_refusal "$home_dir" "$parent_dir" "target basename"
    [[ -e "$parent_dir/SKILL.md" ]] || fail "generic Skill parent was removed: $parent_dir"
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
  skill_dir="$case_root/identity/loomloom"
  write_skill_fixture "$skill_dir"
  sed 's/^name: loomloom$/name: another-skill/' "$skill_dir/SKILL.md" >"$skill_dir/SKILL.tmp"
  mv "$skill_dir/SKILL.tmp" "$skill_dir/SKILL.md"
  expect_shell_skill_refusal "$home_dir" "$skill_dir" "frontmatter"
  [[ -e "$skill_dir" ]] || fail "non-LoomLoom Skill was removed"

  home_dir="$case_root/unrelated/home"
  skill_dir="$case_root/unrelated/loomloom"
  write_skill_fixture "$skill_dir"
  printf 'keep\n' >"$skill_dir/unrelated.txt"
  printf 'hidden\n' >"$skill_dir/.user-settings"
  printf 'special\n' >"$skill_dir/"$'line\nbreak.txt'
  mkdir -p "$skill_dir/custom"
  printf 'nested\n' >"$skill_dir/custom/file.txt"
  output="$(
    env -i HOME="$home_dir" PATH="$test_path" \
      /bin/bash "$repo_root/uninstall.sh" --skill-only --skill-dir "$skill_dir" </dev/null
  )"
  [[ -e "$skill_dir/unrelated.txt" ]] || fail "top-level user file was not preserved"
  [[ -e "$skill_dir/.user-settings" ]] || fail "hidden user file was not preserved"
  [[ -e "$skill_dir/"$'line\nbreak.txt' ]] || fail "user file containing a newline was not preserved"
  [[ -e "$skill_dir/custom/file.txt" ]] || fail "top-level user directory was not preserved"
  [[ ! -e "$skill_dir/SKILL.md" ]] || fail "official SKILL.md was not removed"
  [[ ! -e "$skill_dir/references" ]] || fail "official references directory was not removed"
  [[ ! -e "$skill_dir/generated-template-spec" ]] || fail "official TemplateSpec directory was not removed"
  assert_contains "$output" "unrelated.txt"
  assert_contains "$output" ".user-settings"
  assert_contains "$output" '\nbreak.txt'
  assert_not_contains "$output" $'line\nbreak.txt'
  assert_contains "$output" "non-interactive environment: preserving detected user files by default"

  output="$(
    env -i HOME="$home_dir" PATH="$test_path" \
      /bin/bash "$repo_root/uninstall.sh" --skill-only --skill-dir "$skill_dir" </dev/null
  )"
  [[ -e "$skill_dir/unrelated.txt" ]] || fail "repeat uninstall removed preserved user file"
  assert_contains "$output" "no LoomLoom Skill content found"

  home_dir="$case_root/unreferenced/home"
  skill_dir="$case_root/unreferenced/loomloom"
  write_skill_fixture "$skill_dir"
  printf '# Extra\n' >"$skill_dir/references/extra.md"
  printf '# Extra\n' >"$skill_dir/generated-template-spec/custom-inside.md"
  env -i HOME="$home_dir" PATH="$test_path" \
    /bin/bash "$repo_root/uninstall.sh" --skill-only --skill-dir "$skill_dir" >/dev/null
  [[ ! -e "$skill_dir" ]] || fail "official directories with extra files were not removed"

  home_dir="$case_root/referenced/home"
  skill_dir="$case_root/referenced/parent/loomloom"
  write_skill_fixture "$skill_dir"
  printf '\n- [Extra](references/extra.md#details)\n' >>"$skill_dir/SKILL.md"
  printf '# Extra\n' >"$skill_dir/references/extra.md"
  output="$(
    env -i HOME="$home_dir" PATH="$test_path" \
      /bin/bash "$repo_root/uninstall.sh" --skill-only \
        --skill-dir "$skill_dir/../loomloom/"
  )"
  [[ ! -e "$skill_dir" ]] || fail "new explicitly referenced reference was not removed"
  assert_contains "$output" "removed:"

  home_dir="$case_root/preflight/home"
  install_dir="$case_root/preflight/bin"
  skill_dir="$case_root/preflight/loomloom"
  config_file="$(shell_config_file "$home_dir")"
  write_cli_fixture "$install_dir" loomloom
  write_skill_fixture "$skill_dir"
  sed 's/^name: loomloom$/name: another-skill/' "$skill_dir/SKILL.md" >"$skill_dir/SKILL.tmp"
  mv "$skill_dir/SKILL.tmp" "$skill_dir/SKILL.md"
  mkdir -p "$(dirname "$config_file")"
  printf '{}\n' >"$config_file"
  expect_failure env -i HOME="$home_dir" PATH="$test_path" XDG_CONFIG_HOME="$home_dir/.config" \
    /bin/bash "$repo_root/uninstall.sh" --install-dir "$install_dir" --skill-dir "$skill_dir"
  [[ -e "$install_dir/loomloom" ]] || fail "CLI was removed before Skill validation failed"
  [[ -e "$config_file" ]] || fail "config was removed before Skill validation failed"

  home_dir="$case_root/scan failure/home"
  install_dir="$case_root/scan failure/bin"
  skill_dir="$case_root/scan failure/loomloom"
  config_file="$(shell_config_file "$home_dir")"
  local fake_find_bin="$case_root/scan failure/fake-bin"
  write_cli_fixture "$install_dir" loomloom
  write_skill_fixture "$skill_dir"
  printf 'keep\n' >"$skill_dir/user.txt"
  mkdir -p "$(dirname "$config_file")" "$fake_find_bin"
  printf '%s\n' '#!/usr/bin/env bash' 'exit 23' >"$fake_find_bin/find"
  chmod +x "$fake_find_bin/find"
  printf '{}\n' >"$config_file"
  expect_failure env -i HOME="$home_dir" PATH="$fake_find_bin:$test_path" XDG_CONFIG_HOME="$home_dir/.config" \
    /bin/bash "$repo_root/uninstall.sh" --install-dir "$install_dir" --skill-dir "$skill_dir"
  assert_contains "$failure_output" "cannot enumerate Skill directory"
  [[ -e "$install_dir/loomloom" ]] || fail "CLI was removed after Skill directory scan failed"
  [[ -e "$config_file" ]] || fail "config was removed after Skill directory scan failed"
  [[ -e "$skill_dir/user.txt" ]] || fail "user file was removed after Skill directory scan failed"

  home_dir="$case_root/incomplete/home"
  skill_dir="$case_root/incomplete/loomloom"
  write_skill_fixture "$skill_dir"
  rm -f "$skill_dir/SKILL.md"
  expect_shell_skill_refusal "$home_dir" "$skill_dir" "SKILL.md is missing while"
  [[ -e "$skill_dir/references/setup.md" ]] || fail "incomplete Skill references were removed"

  home_dir="$case_root/generated name collision/home"
  skill_dir="$case_root/generated name collision/loomloom"
  write_skill_fixture "$skill_dir"
  rm -rf "$skill_dir/generated-template-spec"
  printf 'user file\n' >"$skill_dir/generated-template-spec"
  output="$(
    env -i HOME="$home_dir" PATH="$test_path" \
      /bin/bash "$repo_root/uninstall.sh" --skill-only --skill-dir "$skill_dir" </dev/null
  )"
  [[ -f "$skill_dir/generated-template-spec" ]] || fail "generated-template-spec user file was not preserved"
  [[ ! -e "$skill_dir/SKILL.md" && ! -e "$skill_dir/references" ]] || fail "official Skill content was not removed around name collision"

  home_dir="$case_root/generated symlink collision/home"
  skill_dir="$case_root/generated symlink collision/loomloom"
  external_file="$case_root/generated symlink collision/external"
  write_skill_fixture "$skill_dir"
  rm -rf "$skill_dir/generated-template-spec"
  mkdir -p "$external_file"
  printf 'external\n' >"$external_file/keep.txt"
  ln -s "$external_file" "$skill_dir/generated-template-spec"
  output="$(
    env -i HOME="$home_dir" PATH="$test_path" \
      /bin/bash "$repo_root/uninstall.sh" --skill-only --skill-dir "$skill_dir" </dev/null
  )"
  [[ -L "$skill_dir/generated-template-spec" ]] || fail "generated-template-spec user symlink was not preserved"
  [[ -e "$external_file/keep.txt" ]] || fail "generated-template-spec user symlink target was changed"
  assert_contains "$output" "generated-template-spec"

  home_dir="$case_root/internal symlink/home"
  skill_dir="$case_root/internal symlink/loomloom"
  external_file="$case_root/internal symlink/external.txt"
  mkdir -p "$(dirname "$external_file")"
  printf 'external\n' >"$external_file"
  write_skill_fixture "$skill_dir"
  ln -s "$external_file" "$skill_dir/generated-template-spec/external-link"
  env -i HOME="$home_dir" PATH="$test_path" \
    /bin/bash "$repo_root/uninstall.sh" --skill-only --skill-dir "$skill_dir" >/dev/null
  [[ -e "$external_file" ]] || fail "official directory removal followed an internal symbolic link"

  home_dir="$case_root/mutual/home"
  install_dir="$case_root/mutual/bin"
  skill_dir="$case_root/mutual/loomloom"
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

run_shell_interactive_user_file_tests() {
  local expect_path
  expect_path="$(command -v expect || true)"
  if [[ -z "$expect_path" ]]; then
    echo "expect unavailable; skipped interactive uninstall.sh tests"
    return
  fi

  local case_root="$test_root/shell interactive"
  local home_dir="$case_root/home" skill_dir output external_dir

  skill_dir="$case_root/remove user files/loomloom"
  write_skill_fixture "$skill_dir"
  printf 'remove\n' >"$skill_dir/user-file.txt"
  external_dir="$case_root/remove external target"
  mkdir -p "$external_dir"
  printf 'external\n' >"$external_dir/keep.txt"
  rm -rf "$skill_dir/generated-template-spec"
  ln -s "$external_dir" "$skill_dir/generated-template-spec"
  # shellcheck disable=SC2016 # Expect expands these environment variables in the spawned process.
  output="$(
    UNINSTALL_TEST_HOME="$home_dir" \
    UNINSTALL_TEST_PATH="$test_path" \
    UNINSTALL_TEST_SCRIPT="$repo_root/uninstall.sh" \
    UNINSTALL_TEST_SKILL_DIR="$skill_dir" \
    UNINSTALL_TEST_ANSWER="n" \
      "$expect_path" -c '
        set timeout 10
        spawn env -i HOME=$env(UNINSTALL_TEST_HOME) PATH=$env(UNINSTALL_TEST_PATH) /bin/bash $env(UNINSTALL_TEST_SCRIPT) --skill-only --skill-dir $env(UNINSTALL_TEST_SKILL_DIR)
        expect {
          -exact {Keep these files? [Y/n] } {}
          timeout { puts stderr "timed out waiting for uninstall prompt"; exit 124 }
          eof { puts stderr "uninstaller exited before showing prompt"; exit 125 }
        }
        send -- "$env(UNINSTALL_TEST_ANSWER)\r"
        expect {
          eof {}
          timeout { puts stderr "timed out waiting for uninstaller to exit"; exit 126 }
        }
        set result [wait]
        exit [lindex $result 3]
      '
  )"
  [[ ! -e "$skill_dir" ]] || fail "interactive no response did not remove user files"
  [[ -e "$external_dir/keep.txt" ]] || fail "interactive no response followed a user symlink"
  assert_contains "$output" "Keep these files? [Y/n]"
  assert_contains "$output" "removed Skill directory and detected user files:"

  skill_dir="$case_root/keep user files/loomloom"
  write_skill_fixture "$skill_dir"
  printf 'keep\n' >"$skill_dir/user-file.txt"
  mkdir -p "$skill_dir/custom"
  # shellcheck disable=SC2016 # Expect expands these environment variables in the spawned process.
  output="$(
    UNINSTALL_TEST_HOME="$home_dir" \
    UNINSTALL_TEST_PATH="$test_path" \
    UNINSTALL_TEST_SCRIPT="$repo_root/uninstall.sh" \
    UNINSTALL_TEST_SKILL_DIR="$skill_dir" \
    UNINSTALL_TEST_ANSWER="y" \
      "$expect_path" -c '
        set timeout 10
        spawn env -i HOME=$env(UNINSTALL_TEST_HOME) PATH=$env(UNINSTALL_TEST_PATH) /bin/bash $env(UNINSTALL_TEST_SCRIPT) --skill-only --skill-dir $env(UNINSTALL_TEST_SKILL_DIR)
        expect {
          -exact {Keep these files? [Y/n] } {}
          timeout { puts stderr "timed out waiting for uninstall prompt"; exit 124 }
          eof { puts stderr "uninstaller exited before showing prompt"; exit 125 }
        }
        send -- "$env(UNINSTALL_TEST_ANSWER)\r"
        expect {
          eof {}
          timeout { puts stderr "timed out waiting for uninstaller to exit"; exit 126 }
        }
        set result [wait]
        exit [lindex $result 3]
      '
  )"
  [[ -e "$skill_dir/user-file.txt" ]] || fail "interactive yes response did not preserve user file"
  [[ ! -e "$skill_dir/SKILL.md" && ! -e "$skill_dir/references" && ! -e "$skill_dir/generated-template-spec" ]] ||
    fail "interactive yes response did not remove official Skill content"
  assert_contains "$output" "Keep these files? [Y/n]"
  assert_contains "$output" "custom/"

  skill_dir="$case_root/default answer/loomloom"
  write_skill_fixture "$skill_dir"
  printf 'keep\n' >"$skill_dir/user-file.txt"
  # shellcheck disable=SC2016 # Expect expands these environment variables in the spawned process.
  output="$(
    UNINSTALL_TEST_HOME="$home_dir" \
    UNINSTALL_TEST_PATH="$test_path" \
    UNINSTALL_TEST_SCRIPT="$repo_root/uninstall.sh" \
    UNINSTALL_TEST_SKILL_DIR="$skill_dir" \
      "$expect_path" -c '
        set timeout 10
        spawn env -i HOME=$env(UNINSTALL_TEST_HOME) PATH=$env(UNINSTALL_TEST_PATH) /bin/bash $env(UNINSTALL_TEST_SCRIPT) --skill-only --skill-dir $env(UNINSTALL_TEST_SKILL_DIR)
        expect {
          -exact {Keep these files? [Y/n] } {}
          timeout { puts stderr "timed out waiting for uninstall prompt"; exit 124 }
          eof { puts stderr "uninstaller exited before showing prompt"; exit 125 }
        }
        send -- "\r"
        expect {
          eof {}
          timeout { puts stderr "timed out waiting for uninstaller to exit"; exit 126 }
        }
        set result [wait]
        exit [lindex $result 3]
      '
  )"
  [[ -e "$skill_dir/user-file.txt" ]] || fail "interactive default answer did not preserve user file"

  skill_dir="$case_root/invalid then mixed-case answer/loomloom"
  write_skill_fixture "$skill_dir"
  printf 'keep\n' >"$skill_dir/user-file.txt"
  # shellcheck disable=SC2016 # Expect expands these environment variables in the spawned process.
  output="$(
    UNINSTALL_TEST_HOME="$home_dir" \
    UNINSTALL_TEST_PATH="$test_path" \
    UNINSTALL_TEST_SCRIPT="$repo_root/uninstall.sh" \
    UNINSTALL_TEST_SKILL_DIR="$skill_dir" \
      "$expect_path" -c '
        set timeout 10
        spawn env -i HOME=$env(UNINSTALL_TEST_HOME) PATH=$env(UNINSTALL_TEST_PATH) /bin/bash $env(UNINSTALL_TEST_SCRIPT) --skill-only --skill-dir $env(UNINSTALL_TEST_SKILL_DIR)
        expect {
          -exact {Keep these files? [Y/n] } {}
          timeout { puts stderr "timed out waiting for uninstall prompt"; exit 124 }
          eof { puts stderr "uninstaller exited before showing prompt"; exit 125 }
        }
        send -- "invalid\r"
        expect {
          -exact {Please answer yes or no.} {}
          timeout { puts stderr "timed out waiting for validation message"; exit 127 }
          eof { puts stderr "uninstaller exited before validating the answer"; exit 128 }
        }
        expect {
          -exact {Keep these files? [Y/n] } {}
          timeout { puts stderr "timed out waiting for repeated uninstall prompt"; exit 129 }
          eof { puts stderr "uninstaller exited before repeating the prompt"; exit 130 }
        }
        send -- "yEs\r"
        expect {
          eof {}
          timeout { puts stderr "timed out waiting for uninstaller to exit"; exit 126 }
        }
        set result [wait]
        exit [lindex $result 3]
      '
  )"
  [[ -e "$skill_dir/user-file.txt" ]] || fail "mixed-case yes answer did not preserve user file"
  assert_contains "$output" "Please answer yes or no."

  local install_dir="$case_root/cancel/bin"
  local config_file
  home_dir="$case_root/cancel/home"
  skill_dir="$case_root/cancel/skill/loomloom"
  config_file="$(shell_config_file "$home_dir")"
  mkdir -p "$(dirname "$config_file")"
  write_cli_fixture "$install_dir" loomloom
  write_skill_fixture "$skill_dir"
  printf 'keep\n' >"$skill_dir/user-file.txt"
  printf '{}\n' >"$config_file"
  # shellcheck disable=SC2016 # Expect expands these environment variables in the spawned process.
  UNINSTALL_TEST_HOME="$home_dir" \
  UNINSTALL_TEST_XDG_CONFIG_HOME="$home_dir/.config" \
  UNINSTALL_TEST_PATH="$test_path" \
  UNINSTALL_TEST_SCRIPT="$repo_root/uninstall.sh" \
  UNINSTALL_TEST_INSTALL_DIR="$install_dir" \
  UNINSTALL_TEST_SKILL_DIR="$skill_dir" \
    "$expect_path" -c '
      set timeout 10
      spawn env -i HOME=$env(UNINSTALL_TEST_HOME) XDG_CONFIG_HOME=$env(UNINSTALL_TEST_XDG_CONFIG_HOME) PATH=$env(UNINSTALL_TEST_PATH) /bin/bash $env(UNINSTALL_TEST_SCRIPT) --install-dir $env(UNINSTALL_TEST_INSTALL_DIR) --skill-dir $env(UNINSTALL_TEST_SKILL_DIR)
      expect {
        -exact {Keep these files? [Y/n] } {}
        timeout { puts stderr "timed out waiting for uninstall prompt"; exit 124 }
        eof { puts stderr "uninstaller exited before showing prompt"; exit 125 }
      }
      send -- "\003"
      expect {
        eof {}
        timeout { puts stderr "timed out waiting for cancelled uninstaller to exit"; exit 126 }
      }
    ' >/dev/null
  [[ -e "$install_dir/loomloom" ]] || fail "Ctrl-C removed the CLI"
  [[ -e "$skill_dir/SKILL.md" ]] || fail "Ctrl-C removed SKILL.md"
  [[ -e "$skill_dir/references/setup.md" ]] || fail "Ctrl-C removed references"
  [[ -e "$skill_dir/generated-template-spec/manifest.json" ]] || fail "Ctrl-C removed generated TemplateSpec docs"
  [[ -e "$skill_dir/user-file.txt" ]] || fail "Ctrl-C removed a top-level user file"
  [[ -e "$config_file" ]] || fail "Ctrl-C removed config.json"
}

run_powershell_interactive_user_file_tests() {
  local expect_path pwsh_path
  expect_path="$(command -v expect || true)"
  pwsh_path="$(command -v pwsh || true)"
  if [[ -z "$expect_path" || -z "$pwsh_path" ]]; then
    echo "expect or PowerShell unavailable; skipped interactive uninstall.ps1 tests"
    return
  fi

  local case_root="$test_root/powershell interactive"
  local home_dir="$case_root/home" skill_dir output external_dir
  local app_data="$home_dir/AppData/Roaming"

  skill_dir="$case_root/remove user files/loomloom"
  write_skill_fixture "$skill_dir"
  printf 'remove\n' >"$skill_dir/user-file.txt"
  external_dir="$case_root/remove external target"
  mkdir -p "$external_dir"
  printf 'external\n' >"$external_dir/keep.txt"
  rm -rf "$skill_dir/generated-template-spec"
  ln -s "$external_dir" "$skill_dir/generated-template-spec"
  # shellcheck disable=SC2016 # Expect expands these environment variables in the spawned process.
  output="$(
    UNINSTALL_TEST_HOME="$home_dir" \
    UNINSTALL_TEST_APPDATA="$app_data" \
    UNINSTALL_TEST_PATH="$test_path" \
    UNINSTALL_TEST_PWSH="$pwsh_path" \
    UNINSTALL_TEST_SCRIPT="$repo_root/uninstall.ps1" \
    UNINSTALL_TEST_SKILL_DIR="$skill_dir" \
    UNINSTALL_TEST_ANSWER="n" \
      "$expect_path" -c '
        set timeout 10
        spawn env -i HOME=$env(UNINSTALL_TEST_HOME) APPDATA=$env(UNINSTALL_TEST_APPDATA) PATH=$env(UNINSTALL_TEST_PATH) $env(UNINSTALL_TEST_PWSH) -NoProfile -File $env(UNINSTALL_TEST_SCRIPT) -SkillOnly -SkillDir $env(UNINSTALL_TEST_SKILL_DIR)
        expect {
          -exact {Keep these files? [Y/n] } {}
          timeout { puts stderr "timed out waiting for uninstall prompt"; exit 124 }
          eof { puts stderr "uninstaller exited before showing prompt"; exit 125 }
        }
        send -- "$env(UNINSTALL_TEST_ANSWER)\r"
        expect {
          eof {}
          timeout { puts stderr "timed out waiting for uninstaller to exit"; exit 126 }
        }
        set result [wait]
        exit [lindex $result 3]
      '
  )"
  [[ ! -e "$skill_dir" ]] || fail "PowerShell interactive no answer did not remove user files"
  [[ -e "$external_dir/keep.txt" ]] || fail "PowerShell interactive no answer followed a user symlink"
  assert_contains "$output" "Keep these files? [Y/n]"
  assert_contains "$output" "removed Skill directory and detected user files:"

  skill_dir="$case_root/keep user files/loomloom"
  write_skill_fixture "$skill_dir"
  printf 'keep\n' >"$skill_dir/user-file.txt"
  mkdir -p "$skill_dir/custom"
  # shellcheck disable=SC2016 # Expect expands these environment variables in the spawned process.
  output="$(
    UNINSTALL_TEST_HOME="$home_dir" \
    UNINSTALL_TEST_APPDATA="$app_data" \
    UNINSTALL_TEST_PATH="$test_path" \
    UNINSTALL_TEST_PWSH="$pwsh_path" \
    UNINSTALL_TEST_SCRIPT="$repo_root/uninstall.ps1" \
    UNINSTALL_TEST_SKILL_DIR="$skill_dir" \
    UNINSTALL_TEST_ANSWER="y" \
      "$expect_path" -c '
        set timeout 10
        spawn env -i HOME=$env(UNINSTALL_TEST_HOME) APPDATA=$env(UNINSTALL_TEST_APPDATA) PATH=$env(UNINSTALL_TEST_PATH) $env(UNINSTALL_TEST_PWSH) -NoProfile -File $env(UNINSTALL_TEST_SCRIPT) -SkillOnly -SkillDir $env(UNINSTALL_TEST_SKILL_DIR)
        expect {
          -exact {Keep these files? [Y/n] } {}
          timeout { puts stderr "timed out waiting for uninstall prompt"; exit 124 }
          eof { puts stderr "uninstaller exited before showing prompt"; exit 125 }
        }
        send -- "$env(UNINSTALL_TEST_ANSWER)\r"
        expect {
          eof {}
          timeout { puts stderr "timed out waiting for uninstaller to exit"; exit 126 }
        }
        set result [wait]
        exit [lindex $result 3]
      '
  )"
  [[ -e "$skill_dir/user-file.txt" ]] || fail "PowerShell interactive yes answer did not preserve user file"
  [[ ! -e "$skill_dir/SKILL.md" && ! -e "$skill_dir/references" && ! -e "$skill_dir/generated-template-spec" ]] ||
    fail "PowerShell interactive yes answer did not remove official Skill content"
  assert_contains "$output" "Keep these files? [Y/n]"
  assert_contains "$output" "custom/"

  skill_dir="$case_root/default answer/loomloom"
  write_skill_fixture "$skill_dir"
  printf 'keep\n' >"$skill_dir/user-file.txt"
  # shellcheck disable=SC2016 # Expect expands these environment variables in the spawned process.
  output="$(
    UNINSTALL_TEST_HOME="$home_dir" \
    UNINSTALL_TEST_APPDATA="$app_data" \
    UNINSTALL_TEST_PATH="$test_path" \
    UNINSTALL_TEST_PWSH="$pwsh_path" \
    UNINSTALL_TEST_SCRIPT="$repo_root/uninstall.ps1" \
    UNINSTALL_TEST_SKILL_DIR="$skill_dir" \
      "$expect_path" -c '
        set timeout 10
        spawn env -i HOME=$env(UNINSTALL_TEST_HOME) APPDATA=$env(UNINSTALL_TEST_APPDATA) PATH=$env(UNINSTALL_TEST_PATH) $env(UNINSTALL_TEST_PWSH) -NoProfile -File $env(UNINSTALL_TEST_SCRIPT) -SkillOnly -SkillDir $env(UNINSTALL_TEST_SKILL_DIR)
        expect {
          -exact {Keep these files? [Y/n] } {}
          timeout { puts stderr "timed out waiting for uninstall prompt"; exit 124 }
          eof { puts stderr "uninstaller exited before showing prompt"; exit 125 }
        }
        send -- "\r"
        expect {
          eof {}
          timeout { puts stderr "timed out waiting for uninstaller to exit"; exit 126 }
        }
        set result [wait]
        exit [lindex $result 3]
      '
  )"
  [[ -e "$skill_dir/user-file.txt" ]] || fail "PowerShell interactive default answer did not preserve user file"

  skill_dir="$case_root/invalid then mixed-case answer/loomloom"
  write_skill_fixture "$skill_dir"
  printf 'keep\n' >"$skill_dir/user-file.txt"
  # shellcheck disable=SC2016 # Expect expands these environment variables in the spawned process.
  output="$(
    UNINSTALL_TEST_HOME="$home_dir" \
    UNINSTALL_TEST_APPDATA="$app_data" \
    UNINSTALL_TEST_PATH="$test_path" \
    UNINSTALL_TEST_PWSH="$pwsh_path" \
    UNINSTALL_TEST_SCRIPT="$repo_root/uninstall.ps1" \
    UNINSTALL_TEST_SKILL_DIR="$skill_dir" \
      "$expect_path" -c '
        set timeout 10
        spawn env -i HOME=$env(UNINSTALL_TEST_HOME) APPDATA=$env(UNINSTALL_TEST_APPDATA) PATH=$env(UNINSTALL_TEST_PATH) $env(UNINSTALL_TEST_PWSH) -NoProfile -File $env(UNINSTALL_TEST_SCRIPT) -SkillOnly -SkillDir $env(UNINSTALL_TEST_SKILL_DIR)
        expect {
          -exact {Keep these files? [Y/n] } {}
          timeout { puts stderr "timed out waiting for uninstall prompt"; exit 124 }
          eof { puts stderr "uninstaller exited before showing prompt"; exit 125 }
        }
        send -- "invalid\r"
        expect {
          -exact {Please answer yes or no.} {}
          timeout { puts stderr "timed out waiting for validation message"; exit 127 }
          eof { puts stderr "uninstaller exited before validating the answer"; exit 128 }
        }
        expect {
          -exact {Keep these files? [Y/n] } {}
          timeout { puts stderr "timed out waiting for repeated uninstall prompt"; exit 129 }
          eof { puts stderr "uninstaller exited before repeating the prompt"; exit 130 }
        }
        send -- "yEs\r"
        expect {
          eof {}
          timeout { puts stderr "timed out waiting for uninstaller to exit"; exit 126 }
        }
        set result [wait]
        exit [lindex $result 3]
      '
  )"
  [[ -e "$skill_dir/user-file.txt" ]] || fail "PowerShell mixed-case yes answer did not preserve user file"
  assert_contains "$output" "Please answer yes or no."
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
  local skill_dir="$case_root/skill pack/loomloom" config_file="$app_data/loomloom/config.json"
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
  local install_dir="$case_root/bin" skill_dir="$case_root/skill/loomloom"

  expect_failure env -i HOME="$home_dir" APPDATA="$app_data" PATH="$test_path" \
    "$pwsh_path" -NoProfile -File "$repo_root/uninstall.ps1" -SkillOnly
  assert_contains "$failure_output" "-SkillDir is required"

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

  local parent_home="$case_root/generic parent/home" parent_dir
  for parent_dir in \
    "$case_root/custom-runtime/skills" \
    "$case_root/unknown-platform/extensions" \
    "$case_root/agent with spaces/skills"; do
    write_skill_fixture "$parent_dir"
    expect_powershell_skill_refusal "$pwsh_path" "$parent_home" "$parent_dir" "target basename"
    [[ -e "$parent_dir/SKILL.md" ]] || fail "PowerShell removed generic Skill parent: $parent_dir"
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

  local invalid_dir="$case_root/invalid skill/loomloom"
  write_skill_fixture "$invalid_dir"
  sed 's/^name: loomloom$/name: another-skill/' "$invalid_dir/SKILL.md" >"$invalid_dir/SKILL.tmp"
  mv "$invalid_dir/SKILL.tmp" "$invalid_dir/SKILL.md"
  expect_powershell_skill_refusal "$pwsh_path" "$home_dir" "$invalid_dir" "frontmatter"
  [[ -e "$invalid_dir" ]] || fail "PowerShell removed non-LoomLoom Skill"

  local preflight_home="$case_root/preflight/home"
  local preflight_app_data="$preflight_home/AppData/Roaming"
  local preflight_install_dir="$case_root/preflight/bin"
  local preflight_skill_dir="$case_root/preflight/loomloom"
  local preflight_config_file="$preflight_app_data/loomloom/config.json"
  write_cli_fixture "$preflight_install_dir" loomloom.exe
  write_skill_fixture "$preflight_skill_dir"
  sed 's/^name: loomloom$/name: another-skill/' "$preflight_skill_dir/SKILL.md" >"$preflight_skill_dir/SKILL.tmp"
  mv "$preflight_skill_dir/SKILL.tmp" "$preflight_skill_dir/SKILL.md"
  mkdir -p "$(dirname "$preflight_config_file")"
  printf '{}\n' >"$preflight_config_file"
  expect_failure env -i HOME="$preflight_home" APPDATA="$preflight_app_data" PATH="$test_path" \
    "$pwsh_path" -NoProfile -File "$repo_root/uninstall.ps1" \
      -InstallDir "$preflight_install_dir" -SkillDir "$preflight_skill_dir"
  [[ -e "$preflight_install_dir/loomloom.exe" ]] || fail "PowerShell removed CLI before Skill validation failed"
  [[ -e "$preflight_config_file" ]] || fail "PowerShell removed config before Skill validation failed"

  local unrelated_dir="$case_root/unrelated skill/loomloom"
  write_skill_fixture "$unrelated_dir"
  printf 'keep\n' >"$unrelated_dir/unrelated.txt"
  printf 'hidden\n' >"$unrelated_dir/.user-settings"
  printf 'special\n' >"$unrelated_dir/"$'line\nbreak.txt'
  mkdir -p "$unrelated_dir/custom"
  printf 'nested\n' >"$unrelated_dir/custom/file.txt"
  local output
  output="$(
    env -i HOME="$home_dir" APPDATA="$app_data" PATH="$test_path" \
      "$pwsh_path" -NoProfile -File "$repo_root/uninstall.ps1" -SkillOnly -SkillDir "$unrelated_dir" </dev/null
  )"
  [[ -e "$unrelated_dir/unrelated.txt" ]] || fail "PowerShell did not preserve top-level user file"
  [[ -e "$unrelated_dir/.user-settings" ]] || fail "PowerShell did not preserve hidden user file"
  [[ -e "$unrelated_dir/"$'line\nbreak.txt' ]] || fail "PowerShell did not preserve user file containing a newline"
  [[ -e "$unrelated_dir/custom/file.txt" ]] || fail "PowerShell did not preserve top-level user directory"
  [[ ! -e "$unrelated_dir/SKILL.md" ]] || fail "PowerShell did not remove official SKILL.md"
  [[ ! -e "$unrelated_dir/references" ]] || fail "PowerShell did not remove official references"
  [[ ! -e "$unrelated_dir/generated-template-spec" ]] || fail "PowerShell did not remove official TemplateSpec directory"
  assert_contains "$output" "unrelated.txt"
  assert_contains "$output" ".user-settings"
  assert_contains "$output" '\nbreak.txt'
  assert_not_contains "$output" $'line\nbreak.txt'
  assert_contains "$output" "non-interactive environment: preserving detected user files by default"

  output="$(
    env -i HOME="$home_dir" APPDATA="$app_data" PATH="$test_path" \
      "$pwsh_path" -NoProfile -File "$repo_root/uninstall.ps1" -SkillOnly -SkillDir "$unrelated_dir" </dev/null
  )"
  [[ -e "$unrelated_dir/unrelated.txt" ]] || fail "PowerShell repeat uninstall removed user file"
  assert_contains "$output" "no LoomLoom Skill content found"

  local unreferenced_dir="$case_root/unreferenced skill/loomloom"
  write_skill_fixture "$unreferenced_dir"
  printf '# Extra\n' >"$unreferenced_dir/references/extra.md"
  printf '# Extra\n' >"$unreferenced_dir/generated-template-spec/custom-inside.md"
  env -i HOME="$home_dir" APPDATA="$app_data" PATH="$test_path" \
    "$pwsh_path" -NoProfile -File "$repo_root/uninstall.ps1" -SkillOnly -SkillDir "$unreferenced_dir" >/dev/null
  [[ ! -e "$unreferenced_dir" ]] || fail "PowerShell did not remove official directories with extra files"

  local referenced_dir="$case_root/referenced skill/loomloom"
  write_skill_fixture "$referenced_dir"
  printf '\n- [Extra](references/extra.md#details)\n' >>"$referenced_dir/SKILL.md"
  printf '# Extra\n' >"$referenced_dir/references/extra.md"
  env -i HOME="$home_dir" APPDATA="$app_data" PATH="$test_path" \
    "$pwsh_path" -NoProfile -File "$repo_root/uninstall.ps1" -SkillOnly -SkillDir "$referenced_dir" >/dev/null
  [[ ! -e "$referenced_dir" ]] || fail "PowerShell did not remove explicitly referenced reference"

  local incomplete_dir="$case_root/incomplete skill/loomloom"
  write_skill_fixture "$incomplete_dir"
  rm -f "$incomplete_dir/SKILL.md"
  expect_powershell_skill_refusal "$pwsh_path" "$home_dir" "$incomplete_dir" "SKILL.md is missing while"
  [[ -e "$incomplete_dir/references/setup.md" ]] || fail "PowerShell removed incomplete Skill references"

  local generated_collision_dir="$case_root/generated name collision/loomloom"
  write_skill_fixture "$generated_collision_dir"
  rm -rf "$generated_collision_dir/generated-template-spec"
  printf 'user file\n' >"$generated_collision_dir/generated-template-spec"
  output="$(
    env -i HOME="$home_dir" APPDATA="$app_data" PATH="$test_path" \
      "$pwsh_path" -NoProfile -File "$repo_root/uninstall.ps1" -SkillOnly -SkillDir "$generated_collision_dir" </dev/null
  )"
  [[ -f "$generated_collision_dir/generated-template-spec" ]] || fail "PowerShell did not preserve generated-template-spec user file"
  [[ ! -e "$generated_collision_dir/SKILL.md" && ! -e "$generated_collision_dir/references" ]] || fail "PowerShell did not remove official content around name collision"

  local generated_symlink_dir="$case_root/generated symlink collision/loomloom"
  local generated_symlink_target="$case_root/generated symlink external"
  write_skill_fixture "$generated_symlink_dir"
  rm -rf "$generated_symlink_dir/generated-template-spec"
  mkdir -p "$generated_symlink_target"
  printf 'external\n' >"$generated_symlink_target/keep.txt"
  ln -s "$generated_symlink_target" "$generated_symlink_dir/generated-template-spec"
  output="$(
    env -i HOME="$home_dir" APPDATA="$app_data" PATH="$test_path" \
      "$pwsh_path" -NoProfile -File "$repo_root/uninstall.ps1" -SkillOnly -SkillDir "$generated_symlink_dir" </dev/null
  )"
  [[ -L "$generated_symlink_dir/generated-template-spec" ]] || fail "PowerShell did not preserve generated-template-spec user symlink"
  [[ -e "$generated_symlink_target/keep.txt" ]] || fail "PowerShell changed generated-template-spec user symlink target"
  assert_contains "$output" "generated-template-spec"

  local internal_link_dir="$case_root/internal symlink skill/loomloom"
  local external_file="$case_root/internal symlink external.txt"
  printf 'external\n' >"$external_file"
  write_skill_fixture "$internal_link_dir"
  ln -s "$external_file" "$internal_link_dir/generated-template-spec/external-link"
  env -i HOME="$home_dir" APPDATA="$app_data" PATH="$test_path" \
    "$pwsh_path" -NoProfile -File "$repo_root/uninstall.ps1" -SkillOnly -SkillDir "$internal_link_dir" >/dev/null
  [[ -e "$external_file" ]] || fail "PowerShell followed an internal symbolic link while removing official content"

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
run_shell_interactive_user_file_tests
run_powershell_full_uninstall_test
run_powershell_safety_tests
run_powershell_interactive_user_file_tests

echo "uninstall script tests passed"
