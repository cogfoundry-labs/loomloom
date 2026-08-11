#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
SKILL_DIR="$ROOT_DIR/agent-guidance/loomloom"
REFERENCES_DIR="$SKILL_DIR/references"
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
  echo "usage: scripts/skill-references.sh check" >&2
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
  local failed=0 file line_count
  check_exact_files "$REFERENCES_DIR" "canonical Skill reference" || failed=1

  for file in "${REFERENCE_FILES[@]}"; do
    if [[ ! -s "$REFERENCES_DIR/$file" ]]; then
      echo "canonical Skill reference is empty or missing: agent-guidance/loomloom/references/$file" >&2
      failed=1
      continue
    fi
    if ! head -n 1 "$REFERENCES_DIR/$file" | grep -q '^# '; then
      echo "canonical Skill reference must start with an H1 heading: agent-guidance/loomloom/references/$file" >&2
      failed=1
    fi
    line_count=$(wc -l <"$REFERENCES_DIR/$file" | tr -d ' ')
    if (( line_count > 100 )) && ! grep -q '^## Contents$' "$REFERENCES_DIR/$file"; then
      echo "canonical Skill reference over 100 lines must include a Contents section: agent-guidance/loomloom/references/$file" >&2
      failed=1
    fi
  done

  if [[ ! -f "$SKILL_DIR/SKILL.md" ]]; then
    echo "missing LoomLoom SKILL.md: agent-guidance/loomloom/SKILL.md" >&2
    failed=1
  else
    for file in "${REFERENCE_FILES[@]}"; do
      if ! grep -Fq "references/$file" "$SKILL_DIR/SKILL.md"; then
        echo "LoomLoom SKILL.md does not route to references/$file: agent-guidance/loomloom/SKILL.md" >&2
        failed=1
      fi
    done
  fi

  if [[ $failed -ne 0 ]]; then
    return 1
  fi
  echo "LoomLoom Skill references OK"
}

case "${1:-}" in
  check) check_source ;;
  *) usage ;;
esac
