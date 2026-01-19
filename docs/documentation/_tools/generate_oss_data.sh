#!/bin/bash

# Copyright 2024 Flant JSC
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

# This script collects data from all oss.yaml files and generates
# a YAML file with structure: <module-name>: [items]
# Similar to what is done in .werf/defines/oss_yaml.tmpl

if [[ -z ${OSS_SOURCE_DIR} ]]; then
  OSS_SOURCE_DIR=/src
fi

if [[ -z ${OSS_OUTPUT_FILE} ]]; then
  OSS_OUTPUT_FILE=/srv/jekyll-data/documentation/_data/oss.yaml
fi

echo "[] Collecting OSS data from oss.yaml files..."

# Temporary file to collect all module data
TMP_FILE=$(mktemp)
trap "rm -f ${TMP_FILE}" EXIT

# Find all oss.yaml files and process them
while IFS= read -r oss_file; do
  # Skip output file itself and files in _data directories
  if [[ "${oss_file}" == "${OSS_OUTPUT_FILE}" ]] || [[ "${oss_file}" == *"/_data/"* ]]; then
    continue
  fi
  
  # Extract module name from path
  # Examples:
  #   modules/101-cert-manager/oss.yaml -> cert-manager
  #   ee/modules/450-keepalived/oss.yaml -> keepalived
  #   ee/se/modules/380-metallb/oss.yaml -> metallb
  
  # Get directory name
  dir_name=$(dirname "${oss_file}" | xargs basename)
  
  # Remove numeric prefix (XXX-) if present
  module_name=$(echo "${dir_name}" | sed -E 's/^[0-9]{3}-//')
  
  # Skip if module name is empty or only contains numbers
  if [[ -z "${module_name}" ]] || [[ "${module_name}" =~ ^[0-9]+$ ]]; then
    continue
  fi
  
  # Read and parse oss.yaml
  if [[ -f "${oss_file}" ]]; then
    # Use yq to parse YAML and output as JSON, then wrap in module structure
    oss_data=$(yq -o json '.' "${oss_file}" 2>/dev/null || echo '[]')
    
    if [[ "${oss_data}" != "[]" ]] && [[ -n "${oss_data}" ]]; then
      # Create JSON structure: {"module_name": [items]}
      # Use proper jq syntax for dynamic keys - need to escape properly
      echo "${oss_data}" | jq --arg module "${module_name}" '{($module): .}' >> "${TMP_FILE}" 2>/dev/null || {
        # Fallback: create JSON manually if jq fails
        echo "{\"${module_name}\": ${oss_data}}" >> "${TMP_FILE}"
      }
    fi
  fi
done < <(find ${OSS_SOURCE_DIR} -name "oss.yaml" -type f)

# Ensure output directory exists
mkdir -p "$(dirname "${OSS_OUTPUT_FILE}")"

# Merge all module data into single structure
if [[ -f "${TMP_FILE}" ]] && [[ -s "${TMP_FILE}" ]]; then
  # Read all JSON objects and merge them
  jq -s 'add' "${TMP_FILE}" | yq -P '.' > "${OSS_OUTPUT_FILE}"
  echo "OSS data generated successfully: ${OSS_OUTPUT_FILE}"
else
  # Create empty structure
  echo "{}" | yq -P '.' > "${OSS_OUTPUT_FILE}"
  echo "No OSS data found, created empty file: ${OSS_OUTPUT_FILE}"
fi
