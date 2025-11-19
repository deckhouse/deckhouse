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

DMT_VERSION=0.1.54

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

function structure_prepare {
  modules_dir=("ee/modules" "ee/be/modules" "ee/fe/modules" "ee/se/modules" "ee/se-plus/modules")
  candi_cloud_providers_dir=("ee/candi/cloud-providers" "ee/se-plus/candi/cloud-providers")
  cp -R /deckhouse-src /deckhouse
  for dir in "${modules_dir[@]}"; do
    cp -R /deckhouse/"${dir}"/* /deckhouse/modules
  done
  for dir in "${candi_cloud_providers_dir[@]}"; do
    cp -R /deckhouse/"${dir}"/* /deckhouse/candi/cloud-providers
  done
}

apt update > /dev/null
apt install curl -y > /dev/null
structure_prepare
install_dmt
dmt lint -l INFO /deckhouse/modules
