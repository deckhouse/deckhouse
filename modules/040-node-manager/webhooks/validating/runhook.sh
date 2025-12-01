#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <hook-script> <ctx.json>"
  exit 1
fi

HOOK_SCRIPT="$1"
CTX_FILE="$2"

# Load context JSON
CONTEXT="$(cat "$CTX_FILE")"

# Stub: context::jq
context::jq() {
  jq "$@" <<< "$CONTEXT"
}

# Stub: config::jq
config::jq() {
  jq "$@" <<< "{}"
}

# Stub: yq (if needed)
yq() {
  echo "{}" | jq "$@"  # dummy
}

# Where hook writes its result
export VALIDATING_RESPONSE_PATH="./hook_result.json"
rm -f "$VALIDATING_RESPONSE_PATH"

# Stub hook::run → call __main__
hook::run() {
  __main__
}

echo "▶ Running hook: $HOOK_SCRIPT"

# ⛔ вместо запуска отдельного bash-процесса
# bash "$HOOK_SCRIPT"

# ✅ правильный способ — source (в этом же процессе)
source "$HOOK_SCRIPT"

echo ""
echo "=== Hook Result ==="
cat "$VALIDATING_RESPONSE_PATH" | jq .
