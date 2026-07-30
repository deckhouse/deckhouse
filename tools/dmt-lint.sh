#!/usr/bin/env bash

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

set -euo pipefail

# VALIDATION ONLY (deckhouse/dmt#438): build dmt from the PR commit instead of a
# released binary, to run the actualized nelm renderer (public action.ChartRender)
# over all modules. Revert to `DMT_VERSION=<release>` + the curl install before merge.
DMT_REF=97a4944663b412c51dcc7e35a4df3ba2d668c2fc

function install_dmt() {
  GOBIN=/usr/local/bin go install "github.com/deckhouse/dmt/cmd/dmt@${DMT_REF}"
}

function structure_prepare {
  modules_dir=("ee/modules" "ee/be/modules" "ee/fe/modules" "ee/se/modules" "ee/se-plus/modules")
  cloud_providers_glob="030-cloud-provider-*"

  cp -R /deckhouse-src /deckhouse
  mkdir -p /deckhouse/candi/cloud-providers

  for dir in "${modules_dir[@]}"; do
    shopt -s nullglob
    for source_module_dir in /deckhouse/${dir}/*; do
      local module_name
      module_name=$(basename "${source_module_dir}")
      local target_module_dir="/deckhouse/modules/${module_name}"
      local merged_oss_tmp=""

      if [[ -f "${target_module_dir}/oss.yaml" && -f "${source_module_dir}/oss.yaml" ]]; then
        merged_oss_tmp=$(mktemp)
        cat "${target_module_dir}/oss.yaml" > "${merged_oss_tmp}"
        printf "\n" >> "${merged_oss_tmp}"
        cat "${source_module_dir}/oss.yaml" >> "${merged_oss_tmp}"
      fi

      if [[ -d "${target_module_dir}" ]]; then
        cp -R "${source_module_dir}"/. "${target_module_dir}"/
      else
        cp -R "${source_module_dir}" "${target_module_dir}"
      fi

      if [[ -n "${merged_oss_tmp}" ]]; then
        mv "${merged_oss_tmp}" "${target_module_dir}/oss.yaml"
      fi
    done

    for cloud_provider_dir in /deckhouse/${dir}/${cloud_providers_glob}; do
      local cloud_provider_name=$(echo "${cloud_provider_dir}" | grep -oP '(?<=030-cloud-provider-)[^[:space:]]+')
      cp -R $cloud_provider_dir /deckhouse/candi/cloud-providers/"${cloud_provider_name}"
    done
    shopt -u nullglob
  done
}

apt update > /dev/null
apt install curl -y > /dev/null
structure_prepare
install_dmt
dmt lint -l INFO /deckhouse/modules
