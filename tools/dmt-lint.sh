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

DMT_VERSION=0.1.84

function install_dmt() {
  platform_name=$(uname -m)
  os_name=$(uname)

  case "$os_name" in
    Linux)
      local platform="linux"
      ;;
    Darwin)
      local platform="darwin"
      ;;
    *)
      echo "Unsupported OS: $os_name"
      return 1
      ;;
  esac

  case "$platform_name" in
    x86_64)
      local arch="amd64"
      ;;
    arm64)
      local arch="arm64"
      ;;
    aarch64)
      local arch="arm64"
      ;;
    *)
      echo "Unsupported architecture: $platform_name"
      return 1
      ;;
  esac

  curl -sSfL https://github.com/deckhouse/dmt/releases/download/v${DMT_VERSION}/dmt-${DMT_VERSION}-"${platform}"-${arch}.tar.gz | tar -zx --strip-components 1 -C /tmp
  mv /tmp/dmt /usr/local/bin/dmt
  chmod +x /usr/local/bin/dmt

}

function install_yq() {
  platform_name=$(uname -m)
  os_name=$(uname)

  case "$os_name" in
    Linux)
      local platform="linux"
      ;;
    Darwin)
      local platform="darwin"
      ;;
    *)
      echo "Unsupported OS: $os_name"
      return 1
      ;;
  esac

  case "$platform_name" in
    x86_64)
      local arch="amd64"
      ;;
    arm64|aarch64)
      local arch="arm64"
      ;;
    *)
      echo "Unsupported architecture: $platform_name"
      return 1
      ;;
  esac

  curl -sSfL "https://github.com/mikefarah/yq/releases/download/v4.45.1/yq_${platform}_${arch}" -o /usr/local/bin/yq
  chmod +x /usr/local/bin/yq
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
        # Properly merge the two oss.yaml arrays into one via yq
        yq eval-all '[.] | flatten' "${target_module_dir}/oss.yaml" "${source_module_dir}/oss.yaml" > "${merged_oss_tmp}"
        # Check for duplicate object ids in the merged result
        local dup_ids
        dup_ids=$(yq -o=json '[.[].id] | group_by(.) | map(select(length > 1)[0])' "${merged_oss_tmp}")
        if [[ -n "${dup_ids}" && "${dup_ids}" != "[]" ]]; then
          echo "Error: duplicate oss object ids in module ${module_name}: ${dup_ids}"
          exit 1
        fi
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
install_yq
structure_prepare
install_dmt
dmt lint -l INFO /deckhouse/modules
