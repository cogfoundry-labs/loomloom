#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
CORE_DIR=${LOOMLOOM_CORE_DIR:-}
DOCS_DIR="$ROOT_DIR/docs/template-spec"
EN_DIR="$DOCS_DIR/en"
ZH_DIR="$DOCS_DIR/zh-CN"
TRANSLATION_MAP="$DOCS_DIR/translation-map.json"
EMBED_DIR="$ROOT_DIR/cli/internal/template_spec_docs/generated"
SKILL_GENERATED_DIRS=(
  "$ROOT_DIR/skills/loomloom/generated-template-spec"
)

usage() {
  echo "usage: LOOMLOOM_CORE_DIR=/path/to/loomloom scripts/template-spec-docs.sh <record-generation|sync|check|check-local|prepare|prepare-checked|clean|build>" >&2
  exit 2
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command for TemplateSpec docs: $1" >&2
    return 1
  fi
}

translated_sources() {
  find "$CORE_DIR/loomloom-docs/template-spec" -type f \( \
    -name '*.md' -o \
    -path '*/examples/valid/*.json' -o \
    -path '*/examples/invalid/*.json' \
  \) -print0 | LC_ALL=C sort -z
}

rewrite_generated_language_links() {
  local language_dir=$1 file tmp
  if [[ -f "$language_dir/README.md" ]]; then
    tmp=$(mktemp)
    sed 's#](machine/#](../machine/#g; s#`machine/#`../machine/#g' "$language_dir/README.md" >"$tmp"
    mv "$tmp" "$language_dir/README.md"
  fi
  while IFS= read -r -d '' file; do
    tmp=$(mktemp)
    sed 's#](../machine/#](../../machine/#g; s#`../machine/#`../../machine/#g' "$file" >"$tmp"
    mv "$tmp" "$file"
  done < <(find "$language_dir/reference" -type f -name '*.md' -print0)
}

normalize_chinese_generated_source() {
  local source=$1 relative=${1#"$CORE_DIR/loomloom-docs/template-spec/"}
  case "$relative" in
    README.md) sed 's#](machine/#](../machine/#g; s#`machine/#`../machine/#g' "$source" ;;
    reference/*.md) sed 's#](../machine/#](../../machine/#g; s#`../machine/#`../../machine/#g' "$source" ;;
    *) cat "$source" ;;
  esac
}

normalize_chinese_snapshot_source() {
  local source=$1 relative=${1#"$ZH_DIR/"}
  case "$relative" in
    README.md) sed 's#](../machine/#](machine/#g; s#`../machine/#`machine/#g' "$source" ;;
    reference/*.md) sed 's#](../../machine/#](../machine/#g; s#`../../machine/#`../machine/#g' "$source" ;;
    *) cat "$source" ;;
  esac
}

translated_tree_paths() {
  local tree=$1
  find "$tree" -type f \( \
    -name '*.md' -o \
    -path '*/examples/valid/*.json' -o \
    -path '*/examples/invalid/*.json' \
  \) -print | sed "s#^$tree/##" | LC_ALL=C sort
}

record_generation() {
  require_core
  local tmp entries file relative source_digest generated_digest old_source_digest old_generated_digest
  tmp=$(mktemp)
  entries=$(mktemp)
  : >"$entries"
  while IFS= read -r -d '' file; do
    relative=${file#"$CORE_DIR/loomloom-docs/template-spec/"}
    if [[ ! -f "$EN_DIR/$relative" ]]; then
      echo "missing generated English document: $relative" >&2
      rm -f "$tmp" "$entries"
      exit 1
    fi
    source_digest=$(shasum -a 256 "$file" | awk '{print $1}')
    generated_digest=$(shasum -a 256 "$EN_DIR/$relative" | awk '{print $1}')
    if [[ -f "$TRANSLATION_MAP" ]]; then
      old_source_digest=$(jq -r --arg path "$relative" '.files[] | select(.path == $path) | .source_sha256' "$TRANSLATION_MAP")
      old_generated_digest=$(jq -r --arg path "$relative" '.files[] | select(.path == $path) | .generated_sha256' "$TRANSLATION_MAP")
      if [[ -n "$old_source_digest" && "$old_source_digest" != "$source_digest" && "$old_generated_digest" == "$generated_digest" ]]; then
        echo "Chinese source changed but English did not; regenerate English before recording: $relative" >&2
        rm -f "$tmp" "$entries"
        exit 1
      fi
    fi
    jq -n --arg path "$relative" --arg source "$source_digest" --arg generated "$generated_digest" \
      '{path:$path,source_sha256:$source,generated_sha256:$generated}' >>"$entries"
  done < <(translated_sources)
  jq -s '{version:"template-spec-translations/v2",recorder:"template-spec-docs.sh/v1",files:.}' "$entries" >"$tmp"
  mv "$tmp" "$TRANSLATION_MAP"
  rm -f "$entries"
}

check_translation_map() {
  local failed=0 path source expected generated expected_generated mapped_paths source_paths
  if [[ ! -f "$TRANSLATION_MAP" ]]; then
    echo "missing translation-map.json; run record-generation after regenerating English docs" >&2
    return 1
  fi
  while IFS=$'\t' read -r path expected expected_generated; do
    source="$CORE_DIR/loomloom-docs/template-spec/$path"
    generated="$EN_DIR/$path"
    [[ -f "$source" && -f "$generated" ]] || { echo "translation mapping path missing: $path" >&2; failed=1; continue; }
    [[ $(shasum -a 256 "$source" | awk '{print $1}') == "$expected" ]] || { echo "Chinese source changed; regenerate English: $path" >&2; failed=1; }
    [[ $(shasum -a 256 "$generated" | awk '{print $1}') == "$expected_generated" ]] || { echo "English generated file drifted: $path" >&2; failed=1; }
  done < <(jq -r '.files[] | [.path,.source_sha256,.generated_sha256] | @tsv' "$TRANSLATION_MAP")
  [[ $(jq '[.files[].path] | length == (unique | length)' "$TRANSLATION_MAP") == true ]] || { echo "translation mapping contains duplicate paths" >&2; failed=1; }
  mapped_paths=$(jq -r '.files[].path' "$TRANSLATION_MAP" | LC_ALL=C sort)
  source_paths=$(translated_sources | tr '\0' '\n' | sed "s#^$CORE_DIR/loomloom-docs/template-spec/##" | LC_ALL=C sort)
  [[ "$mapped_paths" == "$source_paths" ]] || { echo "translation mapping path set is incomplete" >&2; failed=1; }
  return "$failed"
}

require_core() {
  if [[ -z "$CORE_DIR" || ! -f "$CORE_DIR/loomloom-docs/template-spec/manifest.json" ]]; then
    echo "LOOMLOOM_CORE_DIR must point to the LoomLoom Core canonical docs checkout" >&2
    exit 2
  fi
}

tree_revision() {
  local tree=$1 tmp file relative digest
  tmp=$(mktemp)
  : >"$tmp"
  while IFS= read -r -d '' file; do
    relative=${file#"$tree/"}
    digest=$(shasum -a 256 "$file" | awk '{print $1}')
    printf '%s\0%s\n' "$relative" "$digest" >>"$tmp"
  done < <(find "$tree" -type f ! -name manifest.json -print0 | LC_ALL=C sort -z)
  shasum -a 256 "$tmp" | awk '{print "sha256:" $1}'
  rm -f "$tmp"
}

check_local_json() {
  local failed=0 file
  for file in "$DOCS_DIR/manifest.json" "$TRANSLATION_MAP" \
    "$DOCS_DIR/machine/template-spec.schema.json" \
    "$DOCS_DIR/machine/template-spec-rules.schema.json" \
    "$DOCS_DIR/machine/rules.json"; do
    [[ -f "$file" ]] || { echo "missing TemplateSpec JSON file: ${file#"$ROOT_DIR/"}" >&2; failed=1; continue; }
    jq empty "$file" || failed=1
  done
  while IFS= read -r -d '' file; do
    jq empty "$file" || failed=1
  done < <(find "$EN_DIR/examples" "$ZH_DIR/examples" -type f -name '*.json' -print0)
  return "$failed"
}

check_local_translation_map() {
  local failed=0 path expected_source expected_english english chinese normalized mapped_paths english_paths chinese_paths
  if [[ ! -f "$TRANSLATION_MAP" ]]; then
    echo "missing translation-map.json" >&2
    return 1
  fi
  while IFS=$'\t' read -r path expected_source expected_english; do
    english="$EN_DIR/$path"
    chinese="$ZH_DIR/$path"
    [[ -f "$english" && -f "$chinese" ]] || { echo "local translation path missing: $path" >&2; failed=1; continue; }
    [[ $(shasum -a 256 "$english" | awk '{print $1}') == "$expected_english" ]] || { echo "English generated file drifted: $path" >&2; failed=1; }
    normalized=$(mktemp)
    normalize_chinese_snapshot_source "$chinese" >"$normalized"
    [[ $(shasum -a 256 "$normalized" | awk '{print $1}') == "$expected_source" ]] || { echo "Chinese source snapshot drifted: $path" >&2; failed=1; }
    rm -f "$normalized"
  done < <(jq -r '.files[] | [.path,.source_sha256,.generated_sha256] | @tsv' "$TRANSLATION_MAP")
  [[ $(jq '[.files[].path] | length == (unique | length)' "$TRANSLATION_MAP") == true ]] || { echo "translation mapping contains duplicate paths" >&2; failed=1; }
  mapped_paths=$(jq -r '.files[].path' "$TRANSLATION_MAP" | LC_ALL=C sort)
  english_paths=$(translated_tree_paths "$EN_DIR")
  chinese_paths=$(translated_tree_paths "$ZH_DIR")
  [[ "$mapped_paths" == "$english_paths" ]] || { echo "English document path set drifted from translation map" >&2; failed=1; }
  [[ "$mapped_paths" == "$chinese_paths" ]] || { echo "Chinese document path set drifted from translation map" >&2; failed=1; }
  return "$failed"
}

check_local_docs() {
  local failed=0 expected actual path relative english_norm chinese_norm reference_id anchor
  require_cmd jq || return 1
  require_cmd grep || return 1
  require_cmd shasum || return 1
  check_local_json || failed=1
  check_local_translation_map || failed=1
  while IFS= read -r -d '' path; do
    relative=${path#"$EN_DIR/"}
    english_norm=$(mktemp)
    chinese_norm=$(mktemp)
    normalize_example "$path" >"$english_norm"
    normalize_example "$ZH_DIR/$relative" >"$chinese_norm"
    cmp -s "$english_norm" "$chinese_norm" || { echo "translated example structure drifted: $relative" >&2; failed=1; }
    rm -f "$english_norm" "$chinese_norm"
  done < <(find "$EN_DIR/examples" -type f -name '*.json' -print0)
  expected=$(tree_revision "$DOCS_DIR")
  actual=$(jq -r '.generated_revision' "$DOCS_DIR/manifest.json")
  [[ "$expected" == "$actual" ]] || { echo "generated revision mismatch: manifest=$actual bundle=$expected" >&2; failed=1; }
  [[ $(jq -r '.english_revision' "$DOCS_DIR/manifest.json") == $(tree_revision "$EN_DIR") ]] || { echo "English revision mismatch" >&2; failed=1; }
  [[ $(jq -r '.chinese_revision' "$DOCS_DIR/manifest.json") == $(tree_revision "$ZH_DIR") ]] || { echo "Chinese revision mismatch" >&2; failed=1; }
  jq -e '.owner == "loomloom-docs" and .default_language == "en" and .languages == ["en", "zh-CN"] and (.source_revision | startswith("sha256:"))' "$DOCS_DIR/manifest.json" >/dev/null || { echo "invalid TemplateSpec CLI manifest metadata" >&2; failed=1; }
  while IFS= read -r reference_id; do
    anchor="ref-${reference_id//./-}"
    [[ $(grep -lF -- "<a id=\"$anchor\"></a>" "$EN_DIR/reference"/*.md | wc -l | tr -d ' ') == 1 ]] || { echo "English reference anchor missing or duplicate: $anchor" >&2; failed=1; }
    [[ $(grep -lF -- "<a id=\"$anchor\"></a>" "$ZH_DIR/reference"/*.md | wc -l | tr -d ' ') == 1 ]] || { echo "Chinese reference anchor missing or duplicate: $anchor" >&2; failed=1; }
  done < <(jq -r '.rules[].referenceId' "$DOCS_DIR/machine/rules.json" | sort -u)
  [[ $failed -eq 0 ]] || return 1
  echo "TemplateSpec CLI local docs OK (source=$(jq -r '.source_revision' "$DOCS_DIR/manifest.json") generated=$actual)"
}

sync_docs() {
  require_core
  check_translation_map
  rm -rf "$ZH_DIR"
  mkdir -p "$ZH_DIR"
  cp -R "$CORE_DIR/loomloom-docs/template-spec/README.md" \
    "$CORE_DIR/loomloom-docs/template-spec/concepts" \
    "$CORE_DIR/loomloom-docs/template-spec/get-started" \
    "$CORE_DIR/loomloom-docs/template-spec/how-to" \
    "$CORE_DIR/loomloom-docs/template-spec/reference" \
    "$CORE_DIR/loomloom-docs/template-spec/examples" \
    "$ZH_DIR/"
  rewrite_generated_language_links "$ZH_DIR"
  rm -rf "$DOCS_DIR/machine"
  cp -R "$CORE_DIR/loomloom-docs/template-spec/machine" "$DOCS_DIR/machine"
  local source_revision english_revision chinese_revision generated_revision tmp
  source_revision=$(jq -r '.spec_revision' "$CORE_DIR/loomloom-docs/template-spec/manifest.json")
  english_revision=$(tree_revision "$EN_DIR")
  chinese_revision=$(tree_revision "$ZH_DIR")
  generated_revision=$(tree_revision "$DOCS_DIR")
  tmp=$(mktemp)
  jq -n \
    --arg source "$source_revision" \
    --arg english "$english_revision" \
    --arg chinese "$chinese_revision" \
    --arg generated "$generated_revision" \
    '{owner:"loomloom-docs",source_revision:$source,english_revision:$english,chinese_revision:$chinese,generated_revision:$generated,generator:"loomloom-template-docs/v2",default_language:"en",languages:["en","zh-CN"]}' >"$tmp"
  mv "$tmp" "$DOCS_DIR/manifest.json"
}

check_docs() {
  require_core
  local failed=0 expected actual path reference_id anchor relative core_norm cli_norm source_paths en_paths zh_paths
  check_local_docs || failed=1
  check_translation_map || failed=1
  for path in machine/template-spec.schema.json machine/template-spec-rules.schema.json machine/rules.json; do
    cmp -s "$CORE_DIR/loomloom-docs/template-spec/$path" "$DOCS_DIR/$path" || { echo "machine contract drifted: $path" >&2; failed=1; }
  done
  while IFS= read -r -d '' path; do
    relative=${path#"$CORE_DIR/loomloom-docs/template-spec/"}
    core_norm=$(mktemp)
    cli_norm=$(mktemp)
    normalize_example "$path" >"$core_norm"
    normalize_example "$EN_DIR/$relative" >"$cli_norm"
    cmp -s "$core_norm" "$cli_norm" || { echo "translated example structure drifted: $relative" >&2; failed=1; }
    rm -f "$core_norm" "$cli_norm"
  done < <(find "$CORE_DIR/loomloom-docs/template-spec/examples" -type f -name '*.json' -print0)
  while IFS= read -r -d '' path; do
    relative=${path#"$CORE_DIR/loomloom-docs/template-spec/"}
    core_norm=$(mktemp)
    normalize_chinese_generated_source "$path" >"$core_norm"
    cmp -s "$core_norm" "$ZH_DIR/$relative" || { echo "Chinese generated document drifted: $relative" >&2; failed=1; }
    rm -f "$core_norm"
  done < <(translated_sources)
  source_paths=$(translated_sources | tr '\0' '\n' | sed "s#^$CORE_DIR/loomloom-docs/template-spec/##" | LC_ALL=C sort)
  en_paths=$(find "$EN_DIR" -type f \( -name '*.md' -o -path '*/examples/valid/*.json' -o -path '*/examples/invalid/*.json' \) | sed "s#^$EN_DIR/##" | LC_ALL=C sort)
  zh_paths=$(find "$ZH_DIR" -type f \( -name '*.md' -o -path '*/examples/valid/*.json' -o -path '*/examples/invalid/*.json' \) | sed "s#^$ZH_DIR/##" | LC_ALL=C sort)
  [[ "$source_paths" == "$en_paths" ]] || { echo "English document path set is incomplete" >&2; failed=1; }
  [[ "$source_paths" == "$zh_paths" ]] || { echo "Chinese document path set is incomplete" >&2; failed=1; }
  expected=$(tree_revision "$DOCS_DIR")
  actual=$(jq -r '.generated_revision' "$DOCS_DIR/manifest.json")
  [[ "$expected" == "$actual" ]] || { echo "generated revision mismatch: manifest=$actual bundle=$expected" >&2; failed=1; }
  [[ $(jq -r '.source_revision' "$DOCS_DIR/manifest.json") == $(jq -r '.spec_revision' "$CORE_DIR/loomloom-docs/template-spec/manifest.json") ]] || { echo "source revision mismatch" >&2; failed=1; }
  [[ $(jq -r '.english_revision' "$DOCS_DIR/manifest.json") == $(tree_revision "$EN_DIR") ]] || { echo "English revision mismatch" >&2; failed=1; }
  [[ $(jq -r '.chinese_revision' "$DOCS_DIR/manifest.json") == $(tree_revision "$ZH_DIR") ]] || { echo "Chinese revision mismatch" >&2; failed=1; }
  while IFS= read -r reference_id; do
    anchor="ref-${reference_id//./-}"
    [[ $(grep -lF -- "<a id=\"$anchor\"></a>" "$EN_DIR/reference"/*.md | wc -l | tr -d ' ') == 1 ]] || { echo "English reference anchor missing or duplicate: $anchor" >&2; failed=1; }
    [[ $(grep -lF -- "<a id=\"$anchor\"></a>" "$ZH_DIR/reference"/*.md | wc -l | tr -d ' ') == 1 ]] || { echo "Chinese reference anchor missing or duplicate: $anchor" >&2; failed=1; }
  done < <(jq -r '.rules[].referenceId' "$DOCS_DIR/machine/rules.json" | sort -u)
  [[ $failed -eq 0 ]] || exit 1
  echo "TemplateSpec CLI docs OK (source=$(jq -r '.source_revision' "$DOCS_DIR/manifest.json") generated=$actual)"
}

normalize_example() {
  jq -S '
    def redact($key):
      if has($key) then .[$key] = "__TRANSLATED__" else . end;
    def redact_array($key):
      if has($key) then .[$key] |= map("__TRANSLATED__") else . end;
    .meta |= (redact("name") | redact("description") | redact("scenario") | redact("inputSummary"))
    | .steps |= map(redact("displayName") | redact("instruction"))
    | .inputSchema.fields |= map(
        redact("label") | redact("description") | redact("defaultValue") | redact_array("enumValues")
        | if has("presentation") then
            .presentation |= (redact("placeholder") | redact("hint") | redact_array("examples"))
          else . end
      )
    | .inputSchema |= redact_array("instructions")
    | if .inputSchema | has("sampleRows") then
        .inputSchema.sampleRows |= map(
          if has("values") then .values |= with_entries(.value = "__TRANSLATED__") else . end
        )
      else . end
  ' "$1"
}

clean_docs() {
  find "$EMBED_DIR" -mindepth 1 ! -name placeholder.txt -exec rm -rf {} + 2>/dev/null || true
  for target in "${SKILL_GENERATED_DIRS[@]}"; do rm -rf "$target"; done
}

prepare_docs() {
  clean_docs
  mkdir -p "$EMBED_DIR"
  cp -R "$DOCS_DIR"/. "$EMBED_DIR"/
  for target in "${SKILL_GENERATED_DIRS[@]}"; do mkdir -p "$target"; cp -R "$DOCS_DIR"/. "$target"/; done
}

prepare_checked_docs() {
  check_local_docs
  prepare_docs
}

build_docs() {
  local binary
  binary=$(mktemp)
  trap "clean_docs; rm -f '$binary'" EXIT
  prepare_checked_docs
  (cd "$ROOT_DIR/cli" && go test ./internal/template_spec_docs ./internal/cmd -run 'TestTemplateSpecDocs|TestGeneratedTemplateSpecExamplesAreEnglish|TestGeneratedValidTemplateSpecExamplesPassCLIValidation')
  (cd "$ROOT_DIR/cli" && go build -buildvcs=false -o "$binary" ./cmd/loomloom)
  "$binary" template-spec docs spec >/dev/null
  "$binary" template-spec docs spec --lang zh-CN >/dev/null
}

case "${1:-}" in
  record-generation) record_generation ;;
  sync) sync_docs ;;
  check) check_docs ;;
  check-local) check_local_docs ;;
  prepare) prepare_docs ;;
  prepare-checked) prepare_checked_docs ;;
  clean) clean_docs ;;
  build) build_docs ;;
  *) usage ;;
esac
