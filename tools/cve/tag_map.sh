#!/bin/bash
#
# Copyright 2022 Flant JSC
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
#

set -Eeo pipefail
shopt -s failglob

# This script generates matrix array for workflow in .github/workflow_templates/cve-daily.yml.
# Elements of generated array contains strings that map release tag to one or more corresponding
# release channels in human-readable form, e.g.:
#   ["v1.37.9 => { rock-solid }", "v1.38.4 => { alpha, beta, early-access, stable }"]
# This script requires remote branch corresponding to release channels to be populated, as well as
# release tags.
# This script doesn't take any arguments or env variables.
#
# Usage: tag-map.sh
#
# $SEVERITY - output only entries with specified severity levels (UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL)

channels=("alpha" "beta" "early-access" "stable" "rock-solid")
declare -A tag_map

for channel in "${channels[@]}"; do
  tag=$(git describe --tags --abbrev=0 "origin/$channel")
  if [[ -n "${tag_map[$tag]}" ]]; then
    tag_map[$tag]+=", $channel"
  else
    tag_map[$tag]="$channel"
  fi
done

declare -a matrix_array
for tag in "${!tag_map[@]}"; do
  tag_channels="${tag_map[$tag]}"
  matrix_array+=("$tag => { $tag_channels }")
done

printf -v matrix ', "%s"' "${matrix_array[@]}"
echo "[${matrix:2}]"
