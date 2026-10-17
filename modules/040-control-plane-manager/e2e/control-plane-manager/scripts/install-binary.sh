#!/bin/bash

# Copyright 2026 Flant JSC
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

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=ssh-opts.sh
. "${SCRIPT_DIR}/ssh-opts.sh"

SSH_TARGET="${CLUSTER_SSH_USER}@${CLUSTER_SSH_HOST}"
SSH_OPTS=(
  -o StrictHostKeyChecking=no
  -p "${CLUSTER_SSH_PORT:-22}"
)
JUMP_OPT="$(ssh_proxy_jump_opt)"
if [ -n "${JUMP_OPT}" ]; then
  SSH_OPTS+=(-o "${JUMP_OPT}")
fi

if [ -n "${JUMPHOST_SSH_HOST:-}" ]; then
  echo "Checking required tools on ${CLUSTER_SSH_HOST} via jumphost ${JUMPHOST_SSH_HOST}..."
else
  echo "Checking required tools on ${CLUSTER_SSH_HOST}..."
fi

# chainsaw itself is installed straight from its GitHub release binary
# (linux_amd64) — no Go toolchain needed at all.
# bash -l loads the login profile (PATH additions like /opt/deckhouse/bin for
# yq, etc.) the same way run-tests.sh's `sudo bash -lc` does for the actual
# test run — plain `ssh ... bash -s` is a non-login shell and would miss it.
ssh "${SSH_OPTS[@]}" "${SSH_TARGET}" bash -l -s <<'REMOTE'
set -e

if command -v apt-get >/dev/null 2>&1; then
  PKG_MANAGER=apt
elif command -v dnf >/dev/null 2>&1; then
  PKG_MANAGER=dnf
else
  echo "Unsupported package manager: neither apt-get nor dnf found" >&2
  exit 1
fi
echo "Detected package manager: ${PKG_MANAGER}"

pkg_install() {
  case "${PKG_MANAGER}" in
    apt) sudo apt-get update && sudo apt-get install -y "$@" ;;
    dnf) sudo dnf install -y "$@" ;;
  esac
}

MISSING_PKGS=()
command -v curl  >/dev/null 2>&1 || MISSING_PKGS+=(curl)
command -v tar   >/dev/null 2>&1 || MISSING_PKGS+=(tar)
command -v rsync >/dev/null 2>&1 || MISSING_PKGS+=(rsync)
command -v jq    >/dev/null 2>&1 || MISSING_PKGS+=(jq)
command -v yq    >/dev/null 2>&1 || MISSING_PKGS+=(yq)

if [ "${#MISSING_PKGS[@]}" -gt 0 ]; then
  echo "Installing missing packages: ${MISSING_PKGS[*]}"
  pkg_install "${MISSING_PKGS[@]}"
fi

if ! command -v chainsaw >/dev/null 2>&1; then
  echo "chainsaw not found, installing latest release from GitHub..."
  CHAINSAW_VERSION="$(curl -fsSL https://api.github.com/repos/kyverno/chainsaw/releases/latest | jq -r .tag_name)"
  echo "Installing chainsaw ${CHAINSAW_VERSION} (linux_amd64)..."
  curl -fsSL "https://github.com/kyverno/chainsaw/releases/download/${CHAINSAW_VERSION}/chainsaw_linux_amd64.tar.gz" -o /tmp/chainsaw.tar.gz
  rm -rf /tmp/chainsaw-extract
  mkdir -p /tmp/chainsaw-extract
  tar -C /tmp/chainsaw-extract -xzf /tmp/chainsaw.tar.gz
  sudo install -m 0755 /tmp/chainsaw-extract/chainsaw /usr/local/bin/chainsaw
  rm -rf /tmp/chainsaw.tar.gz /tmp/chainsaw-extract
else
  echo "chainsaw is already installed"
fi

echo -e "\nChainsaw $(chainsaw version | head -n1)"
REMOTE

echo "Binary installation check complete."
