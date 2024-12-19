#!/usr/bin/env bash
set -euo pipefail

DMT_VERSION=0.0.23

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
