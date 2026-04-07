#!/bin/bash

# Copyright 2026 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=log.sh
source "${SCRIPT_DIR}/log.sh"

SOURCE_MODULE=""
SOURCE_ID=""
TARGET_MODULE=""
TARGET_ID=""

SOURCE_OSS_FILE=""
TARGET_OSS_FILE=""

help() {
echo "
Usage: $0 \
  --source-module <module_dir> \
  --source-id <dependency_id> \
  --target-module <module_dir> \
  --target-id <dependency_id>

  Synchronize dependency version data between oss.yaml files of two modules.

Arguments:
  --source-module
      Path to source module directory containing oss.yaml.

  --source-id
      Dependency ID in source oss.yaml.

  --target-module
      Path to target module directory containing oss.yaml.

  --target-id
      Dependency ID in target oss.yaml.

  --help|-h
      Print this message.
"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --source-module)
        shift
        if [[ $# -gt 0 ]]; then
          SOURCE_MODULE="$1"
        else
          log_error "--source-module requires value"
          help
          exit 1
        fi
        ;;
      --source-id)
        shift
        if [[ $# -gt 0 ]]; then
          SOURCE_ID="$1"
        else
          log_error "--source-id requires value"
          help
          exit 1
        fi
        ;;
      --target-module)
        shift
        if [[ $# -gt 0 ]]; then
          TARGET_MODULE="$1"
        else
          log_error "--target-module requires value"
          help
          exit 1
        fi
        ;;
      --target-id)
        shift
        if [[ $# -gt 0 ]]; then
          TARGET_ID="$1"
        else
          log_error "--target-id requires value"
          help
          exit 1
        fi
        ;;
      --help|-h)
        help
        exit 0
        ;;
      *)
        log_error "illegal argument $1"
        help
        exit 1
        ;;
    esac
    shift
  done
}

check_requirements() {
  if ! type yq >/dev/null 2>&1; then
    log_error "yq is required"
    exit 1
  fi

  if [[ -z "$SOURCE_MODULE" ]]; then
    log_error "--source-module is required"
    exit 1
  fi

  if [[ -z "$SOURCE_ID" ]]; then
    log_error "--source-id is required"
    exit 1
  fi

  if [[ -z "$TARGET_MODULE" ]]; then
    log_error "--target-module is required"
    exit 1
  fi

  if [[ -z "$TARGET_ID" ]]; then
    log_error "--target-id is required"
    exit 1
  fi

  SOURCE_OSS_FILE="${SOURCE_MODULE}/oss.yaml"
  TARGET_OSS_FILE="${TARGET_MODULE}/oss.yaml"

  if [[ ! -f "$SOURCE_OSS_FILE" ]]; then
    log_error "source oss.yaml not found: $SOURCE_OSS_FILE"
    exit 1
  fi

  if [[ ! -f "$TARGET_OSS_FILE" ]]; then
    log_error "target oss.yaml not found: $TARGET_OSS_FILE"
    exit 1
  fi
}

check_source_and_target_ids() {
  local source_exists
  local target_exists

  source_exists="$(yq e ".[] | select(.id == \"$SOURCE_ID\") | .id" "$SOURCE_OSS_FILE" | head -n1)"
  target_exists="$(yq e ".[] | select(.id == \"$TARGET_ID\") | .id" "$TARGET_OSS_FILE" | head -n1)"

  [[ -n "$source_exists" && "$source_exists" != "null" ]] || {
    log_error "source id '$SOURCE_ID' not found in $SOURCE_OSS_FILE"
    exit 1
  }

  [[ -n "$target_exists" && "$target_exists" != "null" ]] || {
    log_error "target id '$TARGET_ID' not found in $TARGET_OSS_FILE"
    exit 1
  }
}

sync_dependency_versions() {
  local single_version
  local versions_count

  log_info "sync dependency '$SOURCE_ID' from $SOURCE_OSS_FILE to '$TARGET_ID' in $TARGET_OSS_FILE"

  single_version="$(yq e ".[] | select(.id == \"$SOURCE_ID\") | .version // \"\"" "$SOURCE_OSS_FILE")"
  versions_count="$(yq e ".[] | select(.id == \"$SOURCE_ID\") | (.versions // []) | length" "$SOURCE_OSS_FILE")"

  if [[ -n "$single_version" ]]; then
    yq e -i "
      (.[] | select(.id == \"$TARGET_ID\")).version = \"$single_version\" |
      del((.[] | select(.id == \"$TARGET_ID\")).versions)
    " "$TARGET_OSS_FILE"

    log_info "updated version for dependency '$TARGET_ID' in '$TARGET_OSS_FILE'"
    return 0
  fi

  if [[ "$versions_count" != "0" ]]; then
    yq e -i "
      (.[] | select(.id == \"$TARGET_ID\")).versions = (
        load(\"$SOURCE_OSS_FILE\")[] |
        select(.id == \"$SOURCE_ID\") |
        .versions
      ) |
      del((.[] | select(.id == \"$TARGET_ID\")).version)
    " "$TARGET_OSS_FILE"

    log_info "updated versions for dependency '$TARGET_ID' in '$TARGET_OSS_FILE'"
    return 0
  fi

  log_error "neither version nor versions found for '$SOURCE_ID' in $SOURCE_OSS_FILE"
  exit 1
}

parse_args "$@"
check_requirements
check_source_and_target_ids
sync_dependency_versions
