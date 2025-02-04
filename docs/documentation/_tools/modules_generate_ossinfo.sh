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

#
# Copy files with information about the licenses used in modules to _data/ossinfo folder (jekyll will construct an array with this data)

set -euo pipefail

mkdir -p _data/ossinfo/

> _data/ossinfo-cumulative.yaml

declare -A seen_names=()

for path in $(find "$MODULES_DIR" -iname oss.yaml -print); do
  module_short_name=$(basename "$(dirname "$path")" | cut -d- -f2-)
  cp -f "$path" "_data/ossinfo/${module_short_name}.yaml"

  while IFS= read -r line; do
    if [[ $line =~ ^[[:space:]]*-[[:space:]]name:[[:space:]] ]]; then
      current_name=$(echo "$line" | sed -E 's/.*name:[[:space:]]*"?([^"]+)"?/\1/')
      if [[ -z ${seen_names[$current_name]+x} ]]; then
        seen_names[$current_name]=1
        echo "$line" >> _data/ossinfo-cumulative.yaml
      fi
    else
      echo "$line" >> _data/ossinfo-cumulative.yaml
    fi
  done < "$path"
done
