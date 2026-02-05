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
  # Copy yq from the mounted source directory
  # Resolve symlink to get the actual binary file
  if [ -L /deckhouse-src/bin/yq ]; then
    # Get the symlink target (might be relative or absolute)
    local yq_link=$(readlink /deckhouse-src/bin/yq)
    # If it's an absolute path, try it directly; otherwise resolve relative to bin/
    if [[ "$yq_link" = /* ]]; then
      # Extract just the filename from absolute path
      local yq_file=$(basename "$yq_link")
      local yq_target="/deckhouse-src/bin/$yq_file"
    else
      # Relative path, resolve it relative to bin directory
      local yq_target="/deckhouse-src/bin/$yq_link"
    fi
    
    if [ -f "$yq_target" ]; then
      cp "$yq_target" /usr/local/bin/yq
      chmod +x /usr/local/bin/yq
    else
      echo "Warning: yq target file not found: $yq_target"
    fi
  elif [ -f /deckhouse-src/bin/yq ]; then
    # yq is a regular file, just copy it
    cp /deckhouse-src/bin/yq /usr/local/bin/yq
    chmod +x /usr/local/bin/yq
  else
    echo "Warning: yq not found in /deckhouse-src/bin/"
  fi
}

function copy_with_yaml_merge {
  local src="$1"
  local dst="$2"
  
  # If source is a directory, recursively copy its contents
  if [ -d "$src" ]; then
    mkdir -p "$dst"
    for item in "$src"/*; do
      [ -e "$item" ] || continue
      local basename=$(basename "$item")
      copy_with_yaml_merge "$item" "$dst/$basename"
    done
    return
  fi
  
  # If destination exists and both files are oss.yaml, merge them
  if [ -f "$dst" ] && [ "$(basename "$src")" = "oss.yaml" ]; then
    echo "Merging oss.yaml: $src -> $dst"
    local temp_file=$(mktemp)
    # Concatenate YAML arrays using ireduce
    yq eval-all '. as $item ireduce ([]; . + $item)' "$dst" "$src" > "$temp_file"
    mv "$temp_file" "$dst"
  else
    # Otherwise just copy
    mkdir -p "$(dirname "$dst")"
    cp -f "$src" "$dst"
  fi
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
  
  # Disable dotglob to restore default behavior
  shopt -u dotglob
}

apt update > /dev/null
apt install curl -y > /dev/null
install_yq
structure_prepare
install_dmt
dmt lint -l INFO /deckhouse/modules
