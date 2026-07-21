#!/usr/bin/env bash
set -euo pipefail

ASSETS_DIR="${1:-release}"
CHECKSUMS_FILE="$ASSETS_DIR/checksums.txt"

if [[ ! -d "$ASSETS_DIR" || ! -f "$CHECKSUMS_FILE" ]]; then
  echo "assets directory must contain checksums.txt: $ASSETS_DIR" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  CHECKSUM_COMMAND=(sha256sum)
elif command -v shasum >/dev/null 2>&1; then
  CHECKSUM_COMMAND=(shasum -a 256)
else
  echo "missing required command: sha256sum or shasum" >&2
  exit 1
fi

expected_assets=()
expected_asset_count=0

while IFS= read -r line || [[ -n "$line" ]]; do
  [[ -n "$line" ]] || continue

  checksum="${line%% *}"
  filename="${line#"$checksum"}"
  filename="${filename# }"
  filename="${filename# }"
  filename="${filename#\*}"

  if [[ ! "$checksum" =~ ^[0-9a-f]{64}$ || -z "$filename" ]]; then
    echo "invalid checksum entry: $line" >&2
    exit 1
  fi
  if [[ "$filename" == */* || "$filename" == "." || "$filename" == ".." || "$filename" == "checksums.txt" ]]; then
    echo "invalid release asset name in checksums.txt: $filename" >&2
    exit 1
  fi
  for ((index = 0; index < expected_asset_count; index++)); do
    expected_asset="${expected_assets[$index]}"
    if [[ "$expected_asset" == "$filename" ]]; then
      echo "duplicate release asset in checksums.txt: $filename" >&2
      exit 1
    fi
  done
  if [[ ! -f "$ASSETS_DIR/$filename" ]]; then
    echo "missing release asset listed in checksums.txt: $filename" >&2
    exit 1
  fi

  actual_checksum="$("${CHECKSUM_COMMAND[@]}" "$ASSETS_DIR/$filename" | awk '{print $1}')"
  if [[ "$checksum" != "$actual_checksum" ]]; then
    echo "checksum validation failed for release asset: $filename" >&2
    exit 1
  fi

  expected_assets[expected_asset_count]="$filename"
  expected_asset_count=$((expected_asset_count + 1))
done < "$CHECKSUMS_FILE"

if [[ "$expected_asset_count" -eq 0 ]]; then
  echo "checksums.txt does not list any release assets" >&2
  exit 1
fi

actual_asset_count=0
while IFS= read -r -d '' asset; do
  filename="$(basename "$asset")"
  if [[ "$filename" == "checksums.txt" ]]; then
    continue
  fi
  listed=false
  for ((index = 0; index < expected_asset_count; index++)); do
    expected_asset="${expected_assets[$index]}"
    if [[ "$expected_asset" == "$filename" ]]; then
      listed=true
      break
    fi
  done
  if [[ "$listed" != "true" ]]; then
    echo "release asset is not listed in checksums.txt: $filename" >&2
    exit 1
  fi
  actual_asset_count=$((actual_asset_count + 1))
done < <(find "$ASSETS_DIR" -maxdepth 1 -type f -print0)

if [[ "$actual_asset_count" -ne "$expected_asset_count" ]]; then
  echo "release asset count does not match checksums.txt" >&2
  exit 1
fi

echo "validated $actual_asset_count release assets in $ASSETS_DIR"
