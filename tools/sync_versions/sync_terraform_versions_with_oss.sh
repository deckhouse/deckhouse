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

MODULE=""
TF_FILE="/deckhouse/candi/terraform_versions.yml"
OSS_FILE=""
MODULE_TF_YAML=""
MODULE_TF_DIR=""

help() {
echo "
Usage: $0 --module <cloud_provider_module_dir>

  Synchronize terraform provider versions from oss.yaml to:
    - global candi/terraform_versions.yml
    - module candi/terraform_versions.yml
    - module candi/terraform-modules/version*.tf

Arguments:
  --module
      Path to cloud provider module directory containing oss.yaml.

  --help|-h
      Print this message.
"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --module)
        shift
        if [[ $# -gt 0 ]]; then
          MODULE="$1"
        else
          log_error "--module requires value"
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

  if ! type perl >/dev/null 2>&1; then
    log_error "perl is required"
    exit 1
  fi

  if [[ -z "$MODULE" ]]; then
    log_error "--module is required"
    exit 1
  fi

  OSS_FILE="${MODULE}/oss.yaml"
  MODULE_TF_YAML="${MODULE}/candi/terraform_versions.yml"
  MODULE_TF_DIR="${MODULE}/candi/terraform-modules"

  if [[ ! -f "$OSS_FILE" ]]; then
    log_error "oss.yaml not found: $OSS_FILE"
    exit 1
  fi

  if [[ ! -f "$TF_FILE" ]]; then
    log_error "terraform versions file not found: $TF_FILE"
    exit 1
  fi
}

update_yaml_version() {
  local yaml_file="$1"

  [[ -f "$yaml_file" ]] || {
    log_warn "file not found, skip: $yaml_file"
    return 0
  }

  if [[ -n "$SINGLE_VERSION" ]]; then
    yq e -i "
      .${PROVIDER_ID}.version = \"$SINGLE_VERSION\" |
      del(.${PROVIDER_ID}.versions)
    " "$yaml_file"
  elif [[ "$VERSIONS_COUNT" != "0" ]]; then
    yq e -i "
      .${PROVIDER_ID}.versions = (
        load(\"$OSS_FILE\")[] |
        select(.id == \"$FULL_ID\") |
        [.versions[].version]
      ) |
      del(.${PROVIDER_ID}.version)
    " "$yaml_file"
  else
    log_error "neither version nor versions found for $FULL_ID in $OSS_FILE"
    exit 1
  fi

  log_info "updated YAML: $yaml_file"
}

update_tf_single_version() {
  local tf_path="$1"
  local version="$2"

  [[ -f "$tf_path" ]] || {
    log_warn "file not found, skip: $tf_path"
    return 0
  }

  perl -0pi -e 's/version\s*=\s*"[^"]+"/version = "'"$version"'"/s' "$tf_path"
  log_info "updated TF: $tf_path"
}

update_tf_versions_list() {
  local tf_path="$1"
  shift
  local versions=( "$@" )

  [[ -f "$tf_path" ]] || {
    log_warn "file not found, skip: $tf_path"
    return 0
  }

  local joined=""
  local first=1
  local v

  for v in "${versions[@]}"; do
    if [[ $first -eq 1 ]]; then
      joined="\"$v\""
      first=0
    else
      joined="$joined, \"$v\""
    fi
  done

  if grep -q 'versions\s*=' "$tf_path"; then
    perl -0pi -e 's/versions\s*=\s*\[[^]]*\]/versions = ['"$joined"']/s' "$tf_path"
  else
    perl -0pi -e 's/version\s*=\s*"[^"]+"/versions = ['"$joined"']/s' "$tf_path"
  fi

  log_info "updated TF: $tf_path"
}

update_tf_versions_by_condition() {
  local suffix version tf_path
  while IFS=$'\t' read -r suffix version; do
    [[ -n "$suffix" && "$suffix" != "null" ]] || {
      log_error "condition suffix is empty for $FULL_ID in $OSS_FILE"
      exit 1
    }
    [[ -n "$version" && "$version" != "null" ]] || {
      log_error "version for condition suffix '$suffix' not found in $OSS_FILE"
      exit 1
    }
    tf_path="${MODULE_TF_DIR}/versions-${suffix}.tf"
    update_tf_single_version "$tf_path" "$version"
  done < <(
    yq e -r ".[] |
      select(.id == \"$FULL_ID\") |
      .versions[] |
      select(has(\"condition\")) |
      [((.condition // {}) | to_entries | sort_by(.key) | map(.value) | join(\"-\")), .version] |
      @tsv" "$OSS_FILE"
  )
}

sync_tf_versions() {
  log_info "sync terraform versions with $OSS_FILE"

  FULL_ID="$(yq e '.[] | select(.id | test("^terraform-provider-")) | .id' "$OSS_FILE" | head -n1)"
  [[ -n "$FULL_ID" && "$FULL_ID" != "null" ]] || {
    log_error "terraform provider id not found in $OSS_FILE"
    exit 1
  }

  PROVIDER_ID="${FULL_ID#terraform-provider-}"
  SINGLE_VERSION="$(yq e ".[] | select(.id == \"$FULL_ID\") | .version // \"\"" "$OSS_FILE")"
  VERSIONS_COUNT="$(yq e ".[] | select(.id == \"$FULL_ID\") | (.versions // []) | length" "$OSS_FILE")"
  VERSIONS_LIST="$(yq e ".[] | select(.id == \"$FULL_ID\") | .versions[].version" "$OSS_FILE")"
  CONDITIONS_COUNT="$(yq e ".[] | select(.id == \"$FULL_ID\") | [.versions[]? | select(has(\"condition\"))] | length" "$OSS_FILE")"

  update_yaml_version "$TF_FILE"
  update_yaml_version "$MODULE_TF_YAML"

  if [[ -n "$SINGLE_VERSION" ]]; then
    if [[ "$PROVIDER_ID" == "vcd" ]]; then
      update_tf_single_version "${MODULE_TF_DIR}/versions-legacy.tf" "$SINGLE_VERSION"
      update_tf_single_version "${MODULE_TF_DIR}/versions-new.tf" "$SINGLE_VERSION"
    else
      update_tf_single_version "${MODULE_TF_DIR}/versions.tf" "$SINGLE_VERSION"
    fi
    return 0
  fi

  if [[ "$VERSIONS_COUNT" == "0" ]]; then
    log_error "neither version nor versions found for $FULL_ID in $OSS_FILE"
    exit 1
  fi

  if [[ -z "$VERSIONS_LIST" ]]; then
    log_error "versions list is empty for $FULL_ID in $OSS_FILE"
    exit 1
  fi

  if [[ "$CONDITIONS_COUNT" != "0" ]]; then
    update_tf_versions_by_condition
    return 0
  fi

  local old_ifs="$IFS"
  IFS='
'
  set -- $VERSIONS_LIST
  IFS="$old_ifs"

  update_tf_versions_list "${MODULE_TF_DIR}/versions.tf" "$@"
}

parse_args "$@"
check_requirements
sync_tf_versions
