#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./set-env.sh
source "${SCRIPT_DIR}/set-env.sh"

PROFILE="${1:-all}"

KEY_SIZE="32"
VAL_SIZE="256"
KEYSPACE="100000"
PUT_TOTAL="100000"

run_low() {
  echo "=== PUT: low ==="
  bench \
    --clients=16 \
    --conns=4 \
    put \
    --total="${PUT_TOTAL}" \
    --rate=300 \
    --key-size="${KEY_SIZE}" \
    --val-size="${VAL_SIZE}" \
    --key-space-size="${KEYSPACE}" \
    --sequential-keys
}

run_medium() {
  echo "=== PUT: medium ==="
  bench \
    --clients=32 \
    --conns=8 \
    put \
    --total="${PUT_TOTAL}" \
    --rate=1000 \
    --key-size="${KEY_SIZE}" \
    --val-size="${VAL_SIZE}" \
    --key-space-size="${KEYSPACE}" \
    --sequential-keys
}

run_high() {
  echo "=== PUT: high ==="
  bench \
    --clients=64 \
    --conns=16 \
    put \
    --total="${PUT_TOTAL}" \
    --rate=2500 \
    --key-size="${KEY_SIZE}" \
    --val-size="${VAL_SIZE}" \
    --key-space-size="${KEYSPACE}" \
    --sequential-keys
}

# echo "=== Pre-flight health check ==="
# healthcheck

TZ=Europe/Moscow printf -v start_time "%(%H:%M:%S)T" -1
echo "start time: $start_time"

case "${PROFILE}" in
  low)
    run_low
    ;;
  medium)
    run_medium
    ;;
  high)
    run_high
    ;;
  all)
    run_low
    run_medium
    run_high
    ;;
  *)
    echo "Usage: $0 [low|medium|high|all]" >&2
    exit 1
    ;;
esac

TZ=Europe/Moscow printf -v end_time "%(%H:%M:%S)T" -1
echo "start time: $start_time"
echo "end time: $end_time"

# echo "=== Final health check ==="
# healthcheck

