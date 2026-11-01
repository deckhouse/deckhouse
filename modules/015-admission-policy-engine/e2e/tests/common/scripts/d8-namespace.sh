#!/usr/bin/env bash

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

# Creates or deletes a system namespace (`d8-e2e-ape-*`) for the tests.
#
# Only the Deckhouse service account may create a namespace named `d8-*` or label one with
# `heritage: deckhouse`, so both operations impersonate it (see AS_DECKHOUSE in lib.sh).
#
# Usage:
#   d8-namespace.sh create <NAME> [<KEY>=<VALUE>...]
#   d8-namespace.sh delete <NAME>
set -Eeuo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

action="${1:?action is required}"
name="${2:?namespace name is required}"
shift 2

[[ "${name}" == d8-e2e-ape-* ]] || fail "refusing to manage namespace ${name}: only d8-e2e-ape-* belongs to the suite"

case "${action}" in
  create)
    labels='{}'
    for kv in "$@"; do
      labels="$(jq -c --arg k "${kv%%=*}" --arg v "${kv#*=}" '. + {($k): $v}' <<<"${labels}")"
    done
    if kubectl get namespace "${name}" >/dev/null 2>&1; then
      log "Namespace ${name} is left over from a previous run, deleting it first"
      kubectl delete namespace "${name}" "${AS_DECKHOUSE[@]}" --wait=true --timeout=5m
    fi
    jq -n --arg name "${name}" --argjson labels "${labels}" \
      '{apiVersion: "v1", kind: "Namespace", metadata: {name: $name, labels: $labels}}' \
      | kubectl create "${AS_DECKHOUSE[@]}" -f -
    wait_default_sa "${name}"
    log "Namespace ${name} created with labels ${labels}"
    ;;
  delete)
    kubectl delete namespace "${name}" "${AS_DECKHOUSE[@]}" --ignore-not-found --wait=true --timeout=5m
    log "Namespace ${name} deleted"
    ;;
  *)
    fail "unknown action ${action}"
    ;;
esac
