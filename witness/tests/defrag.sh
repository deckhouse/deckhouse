#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./set-env.sh
source "${SCRIPT_DIR}/set-env.sh"

echo "=== Health before defrag ==="
healthcheck

echo "Defragmenting cluster..."
etcdctl defrag --cluster --command-timeout 30s

echo "=== Cooldown after defrag ==="
sleep 30

echo "=== Health after defrag ==="
healthcheck

