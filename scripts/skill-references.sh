#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
SOURCE_DIR="$ROOT_DIR/skill-sources/references"
TARGET_DIRS=(
  "$ROOT_DIR/skills/loomloom/references"
)
REFERENCE_FILES=(
  billing.md
  cli.md
  execution.md
  local-skills.md
  market.md
  setup.md
  template-spec.md
)

usage() {
  echo "usage: scripts/skill-references.sh <check|prepare|prepare-checked|clean>" >&2
  exit 2
}

expected_file_list() {
  printf '%s\n' "${REFERENCE_FILES[@]}"
}

actual_file_list() {
  local directory=$1
  find "$directory" -mindepth 1 -maxdepth 1 -type f -print \
    | sed "s#^$directory/##" \
    | LC_ALL=C sort
}

check_exact_files() {
  local directory=$1 label=$2 expected actual entry_count
  if [[ ! -d "$directory" ]]; then
    echo "missing $label directory: ${directory#"$ROOT_DIR/"}" >&2
    return 1
  fi
  expected=$(expected_file_list)
  actual=$(actual_file_list "$directory")
  if [[ "$actual" != "$expected" ]]; then
    echo "$label file set does not match the canonical reference list: ${directory#"$ROOT_DIR/"}" >&2
    diff -u <(printf '%s\n' "$expected") <(printf '%s\n' "$actual") >&2 || true
    return 1
  fi
  entry_count=$(find "$directory" -mindepth 1 -maxdepth 1 -print | wc -l | tr -d ' ')
  if [[ "$entry_count" != "${#REFERENCE_FILES[@]}" ]]; then
    echo "$label directory contains unsupported non-file entries: ${directory#"$ROOT_DIR/"}" >&2
    return 1
  fi
}

check_source() {
  local failed=0 file line_count skill_dir
  check_exact_files "$SOURCE_DIR" "canonical Skill reference" || failed=1

  for file in "${REFERENCE_FILES[@]}"; do
    if [[ ! -s "$SOURCE_DIR/$file" ]]; then
      echo "canonical Skill reference is empty or missing: skill-sources/references/$file" >&2
      failed=1
      continue
    fi
    if ! head -n 1 "$SOURCE_DIR/$file" | grep -q '^# '; then
      echo "canonical Skill reference must start with an H1 heading: skill-sources/references/$file" >&2
      failed=1
    fi
    line_count=$(wc -l <"$SOURCE_DIR/$file" | tr -d ' ')
    if (( line_count > 100 )) && ! grep -q '^## Contents$' "$SOURCE_DIR/$file"; then
      echo "canonical Skill reference over 100 lines must include a Contents section: skill-sources/references/$file" >&2
      failed=1
    fi
  done

  for skill_dir in "${TARGET_DIRS[@]}"; do
    if [[ ! -f "${skill_dir%/references}/SKILL.md" ]]; then
      echo "missing LoomLoom SKILL.md: ${skill_dir%/references}/SKILL.md" >&2
      failed=1
      continue
    fi
    for file in "${REFERENCE_FILES[@]}"; do
      if ! grep -Fq "references/$file" "${skill_dir%/references}/SKILL.md"; then
        echo "LoomLoom SKILL.md does not route to references/$file: ${skill_dir%/references}/SKILL.md" >&2
        failed=1
      fi
    done
  done

  if [[ $failed -ne 0 ]]; then
    return 1
  fi
  echo "LoomLoom Skill references OK"
}

clean_references() {
  local target
  for target in "${TARGET_DIRS[@]}"; do
    rm -rf "$target"
  done
}

prepare_references() {
  local target file
  clean_references
  for target in "${TARGET_DIRS[@]}"; do
    mkdir -p "$target"
    for file in "${REFERENCE_FILES[@]}"; do
      cp "$SOURCE_DIR/$file" "$target/$file"
    done
  done
}

verify_generated_references() {
  local failed=0 target file
  for target in "${TARGET_DIRS[@]}"; do
    check_exact_files "$target" "generated Skill reference" || failed=1
    for file in "${REFERENCE_FILES[@]}"; do
      if ! cmp -s "$SOURCE_DIR/$file" "$target/$file"; then
        echo "generated Skill reference differs from canonical source: ${target#"$ROOT_DIR/"}/$file" >&2
        failed=1
      fi
    done
  done
  [[ $failed -eq 0 ]]
}

prepare_checked_references() {
  check_source
  prepare_references
  verify_generated_references
  echo "LoomLoom Skill references prepared"
}

case "${1:-}" in
  check) check_source ;;
  prepare) prepare_references ;;
  prepare-checked) prepare_checked_references ;;
  clean) clean_references ;;
  *) usage ;;
esac
