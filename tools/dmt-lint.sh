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

DMT_VERSION=0.1.67
YQ_VERSION=4.25.3

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

  curl -sSfL https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/yq_${platform}_${arch} -o /usr/local/bin/yq
  chmod +x /usr/local/bin/yq
}

function merge_oss_yaml {
  local src="$1"
  local dst="$2"
  echo "Merging oss.yaml: $src -> $dst"
  local temp_file
  temp_file=$(mktemp)
  # Concatenate YAML arrays using ireduce
  yq eval-all '. as $item ireduce ([]; . + $item)' "$dst" "$src" > "$temp_file"
  mv "$temp_file" "$dst"
}

function structure_prepare {
  modules_dir=("ee/modules" "ee/be/modules" "ee/fe/modules" "ee/se/modules" "ee/se-plus/modules")
  cloud_providers_glob="030-cloud-provider-*"

  cp -R /deckhouse-src /deckhouse
  mkdir -p /deckhouse/candi/cloud-providers

  for dir in "${modules_dir[@]}"; do
    cp -R /deckhouse/"${dir}"/* /deckhouse/modules

    shopt -s nullglob
    for cloud_provider_dir in /deckhouse/${dir}/${cloud_providers_glob}; do
      local cloud_provider_name=$(echo "${cloud_provider_dir}" | grep -oP '(?<=030-cloud-provider-)[^[:space:]]+')
      cp -R $cloud_provider_dir /deckhouse/candi/cloud-providers/"${cloud_provider_name}"
    done
    shopt -u nullglob
  done

  local module="040-terraform-manager"
  local dst="/deckhouse/modules/${module}/oss.yaml"
  local base="/deckhouse-src/modules/${module}/oss.yaml"

  if [ -f "$base" ]; then
    mkdir -p "$(dirname "$dst")"
    cp -f "$base" "$dst"
  fi

  for dir in "${modules_dir[@]}"; do
    local src="/deckhouse/${dir}/${module}/oss.yaml"
    if [ -f "$src" ]; then
      merge_oss_yaml "$src" "$dst"
    fi
  done
  
  # Disable dotglob to restore default behavior
  shopt -u dotglob
}

apt update > /dev/null
apt install curl -y > /dev/null
install_yq
structure_prepare
install_dmt
dmt lint -l INFO /deckhouse/modules
