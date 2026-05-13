#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./set-env.sh
source "${SCRIPT_DIR}/set-env.sh"

echo "=== Health before compact ==="
healthcheck

rev="$(
  etcdctl endpoint status -w json \
    | jq -r 'map(.Status.header.revision) | max'
)"

echo "Compacting at revision: ${rev}"
etcdctl compact "${rev}"

echo "=== Cooldown after compact ==="
sleep 20

echo "=== Health after compact ==="
healthcheck
