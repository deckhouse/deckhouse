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

# jq/yq are assumed already present for root on the target host, so the only
# thing this installs is chainsaw (via `go install`), and Go itself if it's
# missing. Go is fetched straight from go.dev (linux-amd64), not the distro
# package manager: distro Go packages lag well behind upstream and are
# routinely too old to build current chainsaw releases.
ssh "${SSH_OPTS[@]}" "${SSH_TARGET}" bash -s <<'REMOTE'
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

if ! command -v go >/dev/null 2>&1; then
  echo "go not found, installing latest release from go.dev..."
  command -v curl >/dev/null 2>&1 || pkg_install curl
  command -v tar >/dev/null 2>&1 || pkg_install tar
  GO_VERSION="$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -n1)"
  echo "Installing ${GO_VERSION} (linux-amd64)..."
  curl -fsSL "https://go.dev/dl/${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tar.gz
  sudo rm -rf /usr/local/go
  sudo tar -C /usr/local -xzf /tmp/go.tar.gz
  rm -f /tmp/go.tar.gz
  # Symlink into /usr/local/bin (already on PATH everywhere) so `go` resolves
  # for the rest of this script and for every later run/session, the same way
  # chainsaw itself gets symlinked below.
  sudo ln -sf /usr/local/go/bin/go /usr/local/bin/go
  sudo ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
else
  echo "go is already installed ($(go version))"
fi

if ! command -v chainsaw >/dev/null 2>&1; then
  echo "chainsaw not found, installing..."
  go install github.com/kyverno/chainsaw@latest
  sudo ln -sf "$(go env GOPATH)/bin/chainsaw" /usr/local/bin/chainsaw
else
  echo "chainsaw is already installed"
fi


echo -e  "\n$(go version)\nChainsaw $(chainsaw version | head -n1)"
REMOTE

echo "Binary installation check complete."
