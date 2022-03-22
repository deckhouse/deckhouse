#!/bin/bash

# Copyright 2021 Flant JSC
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

set -euo pipefail

version_constraint_prefix="#kernel-version-constraint "
actual_config_path="/tmp/actual-config.yaml"

correct_kernel_version_code="0"
wrong_kernel_version_code="13"

echo "programs:" > "$actual_config_path"

for program_path in /config/*.yaml; do
    program_body="$(cat "$program_path")"

    raw_constraints="$(grep "$version_constraint_prefix" <<<"$program_body")"
    constraints="$(sed "s/$version_constraint_prefix//g" <<<"$raw_constraints")"

    ret="$(kernel-version-parser "$constraints"; echo $?)"

    if [[ "$ret" == "$correct_kernel_version_code" ]]; then
        echo "$program_body" >> "$actual_config_path"
    elif [[ "$ret" != "$wrong_kernel_version_code" ]]; then
        exit 1
    fi
done

/usr/local/bin/ebpf_exporter "$@"
