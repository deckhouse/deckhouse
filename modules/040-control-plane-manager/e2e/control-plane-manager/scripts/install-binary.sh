#!/bin/bash
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

ssh "${SSH_OPTS[@]}" "${SSH_TARGET}" \
    "set -e; \
     if ! command -v jq >/dev/null 2>&1; then \
         echo 'jq not found, installing...'; \
         sudo apt-get update && sudo apt-get install -y jq; \
     else \
         echo 'jq is already installed'; \
     fi; \
     if ! command -v yq >/dev/null 2>&1; then \
         echo 'yq not found, installing...'; \
         if ! command -v go >/dev/null 2>&1; then \
             echo 'Go not found, installing...'; \
             sudo apt-get update && sudo apt-get install -y golang-go; \
         fi; \
         export PATH=\$PATH:/usr/local/go/bin:\$(go env GOPATH)/bin; \
         go install github.com/mikefarah/yq/v4@latest; \
         sudo ln -sf \$(go env GOPATH)/bin/yq /usr/local/bin/yq; \
     else \
         echo 'yq is already installed'; \
     fi; \
     if ! command -v chainsaw >/dev/null 2>&1; then \
         echo 'chainsaw not found, checking for Go...'; \
         if ! command -v go >/dev/null 2>&1; then \
             echo 'Go not found, installing...'; \
             sudo apt-get update && sudo apt-get install -y golang-go; \
         fi; \
         export PATH=\$PATH:/usr/local/go/bin:\$(go env GOPATH)/bin; \
         echo 'Installing chainsaw...'; \
         go install github.com/kyverno/chainsaw@latest; \
         sudo ln -sf \$(go env GOPATH)/bin/chainsaw /usr/local/bin/chainsaw; \
     else \
         echo 'chainsaw is already installed'; \
     fi"

echo "Binary installation check complete."
