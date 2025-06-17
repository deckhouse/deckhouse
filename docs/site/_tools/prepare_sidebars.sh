#!/bin/bash

# Copyright 2025 Flant JSC
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

PATH_TO_SIDEBARS="_data/sidebars"
PATH_TO_DATA="_data"

urls=($(yq '.. | select(has("url")) | .url?' $PATH_TO_SIDEBARS/virtualization-platform.yml))
declare -a br_array=()
for url in "${urls[@]}"; do
    br_array+=($(dirname $url))
done
breadcrumbs_urls=($(printf "%s\n" "${br_array[@]}" | tr ' ' '\n' | awk '!u[$0]++' | tr '\n' ' '))

for br in "${breadcrumbs_urls[@]}"; do
    echo $br
done



