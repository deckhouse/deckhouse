#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./set-env.sh
source "${SCRIPT_DIR}/set-env.sh"

PROFILE="${1:-all}"

RANGE_START="/registry/"
RANGE_END="/registry0"
RANGE_TOTAL="100000"
RANGE_LIMIT="100"
RANGE_CONSISTENCY="l"

run_low() {
  echo "=== RANGE: low ==="
  bench \
    --clients=16 \
    --conns=4 \
    range "${RANGE_START}" "${RANGE_END}" \
    --total="${RANGE_TOTAL}" \
    --rate=1000 \
    --consistency="${RANGE_CONSISTENCY}" \
    --limit="${RANGE_LIMIT}"
}

run_medium() {
  echo "=== RANGE: medium ==="
  bench \
    --clients=32 \
    --conns=8 \
    range "${RANGE_START}" "${RANGE_END}" \
    --total="${RANGE_TOTAL}" \
    --rate=3000 \
    --consistency="${RANGE_CONSISTENCY}" \
    --limit="${RANGE_LIMIT}"
}

run_high() {
  echo "=== RANGE: high ==="
  bench \
    --clients=64 \
    --conns=16 \
    range "${RANGE_START}" "${RANGE_END}" \
    --total="${RANGE_TOTAL}" \
    --rate=8000 \
    --consistency="${RANGE_CONSISTENCY}" \
    --limit="${RANGE_LIMIT}"
}

echo "=== Pre-flight health check ==="
healthcheck

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
    sleep 20
    run_medium
    sleep 20
    run_high
    ;;
  *)
    echo "Usage: $0 [low|medium|high|all]" >&2
    exit 1
    ;;
esac

TZ=Europe/Moscow printf -v end_time "%(%H:%M:%S)T" -1
echo "start time: $start_time"
echo "end time $end_time"

echo "=== Final health check ==="
healthcheck

