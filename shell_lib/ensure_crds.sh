#!/bin/bash

# Copyright 2021 Flant CJSC
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

function common_hooks::https::ensure_crds::config() {
  cat << EOF
    configVersion: v1
    onStartup: 10
EOF
}

function common_hooks::https::ensure_crds::main() {
  custom_fields_regexp="(x-description|x-doc-default)"

  crds=$(for file in "$@"; do
    name=$(basename -- "$file")
    if [[ $name == doc-* ]]; then
      continue
    fi

    echo "---";
    # Prune custom fields
    cat "$file"
  done)

  readarray -t -d $'\n' crds_json < <(yq r -d '*' - --tojson <<<"$crds" \
    | jq -rc --arg regex "$custom_fields_regexp" '
      .[] | select(.)
      | walk(
        if type == "object"
        then with_entries(
          select(.key | test($regex) | not)
        )
        else . end)')

  cluster_crds="$(kubectl get crds -o json | jq '.items[]')"

  for crd in "${crds_json[@]}"; do
    crd_name="$(jq -er '.metadata.name' <<< "$crd")"

    if cluster_crd="$(jq -re --arg name "$crd_name" 'select(.metadata.name | contains($name)) | select(.spec.conversion)' <<< "$cluster_crds")"; then
      crd="$(jq -re --slurpfile cluster_crd <(printf "%s" "$cluster_crd") '.spec.conversion = $cluster_crd[0].spec.conversion' <<<"$crd")"
    fi

    kubernetes::replace_or_create_json <<< "$crd"
  done
}
