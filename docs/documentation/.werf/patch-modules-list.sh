#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 3 ]]; then
  echo "Usage: $0 <common_list_path> <version> <version_list_path>" >&2
  exit 1
fi

common_list_path="$1"
version="$2"
version_list_path="$3"
changes_file_path="${common_list_path}-changes.json"

if [[ ! -f "$version_list_path" ]]; then
  echo "Version list file does not exist: $version_list_path" >&2
  exit 1
fi

if ! jq -e 'type == "array"' "$version_list_path" >/dev/null; then
  echo "Version list must be a JSON array: $version_list_path" >&2
  exit 1
fi

common_dir="$(dirname "$common_list_path")"
mkdir -p "$common_dir"

if [[ ! -f "$common_list_path" ]]; then
  echo '{}' >"$common_list_path"
fi

if ! jq -e 'type == "object"' "$common_list_path" >/dev/null; then
  echo "Common list must be a JSON object: $common_list_path" >&2
  exit 1
fi

rm -f "$changes_file_path"

if jq -e --arg v "$version" --slurpfile new "$version_list_path" '(.[$v] // null) == $new[0]' "$common_list_path" >/dev/null; then
  echo "No changes for version '$version'"
  exit 0
fi

jq --arg v "$version" --slurpfile new "$version_list_path" '.[$v] = $new[0]' "$common_list_path" >"$changes_file_path"

echo "Updated modules list for version '$version'"
