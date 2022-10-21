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
