#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
test_root="$(mktemp -d)"
trap 'rm -rf "$test_root"' EXIT

fail() {
  echo "install test failed: $*" >&2
  exit 1
}

assert_file_contains() {
  local file="$1" expected="$2"
  [[ -f "$file" ]] || fail "missing file: $file"
  grep -Fq -- "$expected" "$file" || fail "file does not contain '$expected': $file"
}

expect_failure() {
  local output status
  set +e
  output="$("$@" 2>&1)"
  status=$?
  set -e
  [[ "$status" -ne 0 ]] || fail "command unexpectedly succeeded: $*"
  printf '%s\n' "$output"
}

write_skill_fixture() {
  local skill_dir="$1" name="$2"
  mkdir -p "$skill_dir/references"
  printf '%s\n' \
    '---' \
    "name: $name" \
    'description: installer test Skill' \
    '---' \
    '# Installer test Skill' \
    '' \
    '- [Setup](references/setup.md)' >"$skill_dir/SKILL.md"
  printf '# Setup\n' >"$skill_dir/references/setup.md"
}

missing_install_dir="$test_root/missing install dir"
output="$(expect_failure /bin/bash "$repo_root/install.sh" --install-dir "$missing_install_dir")"
[[ "$output" == *"--skill-dir is required"* ]] || fail "install.sh did not require --skill-dir"
[[ ! -e "$missing_install_dir" ]] || fail "install.sh wrote files before validating --skill-dir"

output="$(expect_failure /bin/bash "$repo_root/install.sh" --skill-dir "$test_root/not-loomloom")"
[[ "$output" == *"ending in /loomloom"* ]] || fail "install.sh accepted an incomplete Skill directory"

other_skill_dir="$test_root/official other skill/loomloom"
other_install_dir="$test_root/official other bin"
write_skill_fixture "$other_skill_dir" "another-skill"
output="$(expect_failure /bin/bash "$repo_root/install.sh" \
  --install-dir "$other_install_dir" \
  --skill-dir "$other_skill_dir")"
[[ "$output" == *"refusing to overwrite an existing non-LoomLoom Skill"* ]] || fail "install.sh accepted another Skill"
assert_file_contains "$other_skill_dir/SKILL.md" "name: another-skill"
[[ ! -e "$other_install_dir" ]] || fail "install.sh wrote files before rejecting another Skill"

missing_local_install_dir="$test_root/missing local install dir"
output="$(expect_failure /bin/bash "$repo_root/scripts/install-local.sh" --install-dir "$missing_local_install_dir")"
[[ "$output" == *"--skill-dir is required"* ]] || fail "install-local.sh did not require --skill-dir"
[[ ! -e "$missing_local_install_dir" ]] || fail "install-local.sh wrote files before validating --skill-dir"

other_local_skill_dir="$test_root/local other skill/loomloom"
other_local_install_dir="$test_root/local other bin"
write_skill_fixture "$other_local_skill_dir" "another-skill"
output="$(expect_failure /bin/bash "$repo_root/scripts/install-local.sh" \
  --install-dir "$other_local_install_dir" \
  --skill-dir "$other_local_skill_dir")"
[[ "$output" == *"refusing to overwrite an existing non-LoomLoom Skill"* ]] || fail "install-local.sh accepted another Skill"
assert_file_contains "$other_local_skill_dir/SKILL.md" "name: another-skill"
[[ ! -e "$other_local_install_dir" ]] || fail "install-local.sh wrote files before rejecting another Skill"

local_install_dir="$test_root/local bin"
local_skill_dir="$test_root/agent with spaces/skills/loomloom"
/bin/bash "$repo_root/scripts/install-local.sh" \
  --install-dir "$local_install_dir" \
  --skill-dir "$local_skill_dir" >/dev/null

[[ -x "$local_install_dir/loomloom" ]] || fail "install-local.sh did not install the CLI"
[[ -f "$local_skill_dir/SKILL.md" ]] || fail "install-local.sh did not install the unified Skill"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in
  arm64|aarch64) arch="arm64" ;;
  x86_64|amd64) arch="amd64" ;;
  *) fail "unsupported test architecture: $arch" ;;
esac

release_assets="$test_root/release assets"
release_cli_stage="$test_root/release cli"
release_skills_stage="$test_root/release skills"
fake_bin="$test_root/fake bin"
mkdir -p "$release_assets" "$release_cli_stage" "$release_skills_stage/skills" "$fake_bin"
cp /usr/bin/true "$release_cli_stage/loomloom"
chmod +x "$release_cli_stage/loomloom"
write_skill_fixture "$release_skills_stage/skills/loomloom" "loomloom"
tar -czf "$release_assets/loomloom-$os-$arch.tar.gz" -C "$release_cli_stage" loomloom
tar -czf "$release_assets/loomloom-skills.tar.gz" -C "$release_skills_stage" skills
: >"$release_assets/checksums.txt"

printf '%s\n' \
  '#!/usr/bin/env bash' \
  'set -euo pipefail' \
  'output=""' \
  'url=""' \
  'while [[ $# -gt 0 ]]; do' \
  '  case "$1" in' \
  '    -o) output="$2"; shift 2 ;;' \
  '    -*) shift ;;' \
  '    *) url="$1"; shift ;;' \
  '  esac' \
  'done' \
  '[[ -n "$output" && -n "$url" ]]' \
  'cp "$INSTALL_TEST_ASSET_DIR/${url##*/}" "$output"' >"$fake_bin/curl"
chmod +x "$fake_bin/curl"

official_install_dir="$test_root/official bin"
official_skill_dir="$test_root/custom-runtime/skills/loomloom"
write_skill_fixture "$official_skill_dir" "loomloom"
printf 'preserve\n' >"$official_skill_dir/user-file.txt"
env PATH="$fake_bin:$PATH" INSTALL_TEST_ASSET_DIR="$release_assets" \
  /bin/bash "$repo_root/install.sh" \
    --no-brew \
    --version v0.0.0-test \
    --install-dir "$official_install_dir" \
    --skill-dir "$official_skill_dir" >/dev/null

[[ -x "$official_install_dir/loomloom" ]] || fail "install.sh did not install the CLI"
assert_file_contains "$official_skill_dir/SKILL.md" "name: loomloom"
[[ -f "$official_skill_dir/references/setup.md" ]] || fail "install.sh did not install unified Skill references"
[[ -f "$official_skill_dir/user-file.txt" ]] || fail "install.sh did not preserve an existing LoomLoom user file"

pwsh_path="$(command -v pwsh || true)"
if [[ -n "$pwsh_path" ]]; then
  output="$(expect_failure "$pwsh_path" -NoProfile -File "$repo_root/install.ps1")"
  [[ "$output" == *"-SkillDir is required"* ]] || fail "install.ps1 did not require -SkillDir"

  output="$(expect_failure "$pwsh_path" -NoProfile -File "$repo_root/install.ps1" -SkillDir "$test_root/not-loomloom")"
  [[ "$output" == *"complete LoomLoom Skill directory"* ]] || fail "install.ps1 accepted an incomplete Skill directory"

  other_ps_skill_dir="$test_root/powershell other skill/loomloom"
  other_ps_install_dir="$test_root/powershell other bin"
  write_skill_fixture "$other_ps_skill_dir" "another-skill"
  output="$(expect_failure "$pwsh_path" -NoProfile -File "$repo_root/install.ps1" \
    -InstallDir "$other_ps_install_dir" \
    -SkillDir "$other_ps_skill_dir")"
  [[ "$output" == *"refusing to overwrite an existing non-LoomLoom Skill"* ]] || fail "install.ps1 accepted another Skill"
  assert_file_contains "$other_ps_skill_dir/SKILL.md" "name: another-skill"
  [[ ! -e "$other_ps_install_dir" ]] || fail "install.ps1 wrote files before rejecting another Skill"

  ps_release_assets="$test_root/powershell release assets"
  ps_cli_stage="$test_root/powershell release cli"
  ps_skills_stage="$test_root/powershell release skills"
  mkdir -p "$ps_release_assets" "$ps_cli_stage" "$ps_skills_stage/skills"
  cp /usr/bin/true "$ps_cli_stage/loomloom.exe"
  write_skill_fixture "$ps_skills_stage/skills/loomloom" "loomloom"
  : >"$ps_release_assets/checksums.txt"

  INSTALL_TEST_ZIP_SOURCE="$ps_cli_stage" \
  INSTALL_TEST_ZIP_DEST="$ps_release_assets/loomloom-windows-amd64.zip" \
    "$pwsh_path" -NoProfile -Command \
      'Compress-Archive -Path (Join-Path $env:INSTALL_TEST_ZIP_SOURCE "*") -DestinationPath $env:INSTALL_TEST_ZIP_DEST -Force'
  INSTALL_TEST_ZIP_SOURCE="$ps_skills_stage" \
  INSTALL_TEST_ZIP_DEST="$ps_release_assets/loomloom-skills.zip" \
    "$pwsh_path" -NoProfile -Command \
      'Compress-Archive -Path (Join-Path $env:INSTALL_TEST_ZIP_SOURCE "*") -DestinationPath $env:INSTALL_TEST_ZIP_DEST -Force'

  ps_install_dir="$test_root/powershell bin"
  ps_skill_dir="$test_root/unknown-platform/extensions/loomloom"
  INSTALL_TEST_PS_ASSETS="$ps_release_assets" \
  INSTALL_TEST_PS_SCRIPT="$repo_root/install.ps1" \
  INSTALL_TEST_PS_INSTALL_DIR="$ps_install_dir" \
  INSTALL_TEST_PS_SKILL_DIR="$ps_skill_dir" \
  PROCESSOR_ARCHITECTURE="AMD64" \
    "$pwsh_path" -NoProfile -Command '
      function Invoke-WebRequest {
        param([string]$Uri, [string]$OutFile)
        $asset = Join-Path $env:INSTALL_TEST_PS_ASSETS ([System.IO.Path]::GetFileName($Uri))
        Copy-Item -LiteralPath $asset -Destination $OutFile
      }
      & $env:INSTALL_TEST_PS_SCRIPT `
        -Version v0.0.0-test `
        -InstallDir $env:INSTALL_TEST_PS_INSTALL_DIR `
        -SkillDir $env:INSTALL_TEST_PS_SKILL_DIR
    ' >/dev/null

  [[ -f "$ps_install_dir/loomloom.exe" ]] || fail "install.ps1 did not install the CLI"
  assert_file_contains "$ps_skill_dir/SKILL.md" "name: loomloom"
  [[ -f "$ps_skill_dir/references/setup.md" ]] || fail "install.ps1 did not install unified Skill references"
else
  echo "PowerShell unavailable; skipped install.ps1 parameter tests"
fi

echo "install script tests passed"
