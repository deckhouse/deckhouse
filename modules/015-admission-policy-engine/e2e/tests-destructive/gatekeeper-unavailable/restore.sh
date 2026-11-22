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

# Scales the Gatekeeper webhook back to the replicas take-down.sh recorded and waits until
# admission works again. Idempotent: the test runs it both after the outage and in cleanup.
#
# The record is removed as soon as the Deployment has its replicas back, so a wait that times out
# afterwards does not leave it for a later run. The waits add up to about 11 minutes at most,
# inside the 12m timeout of the finally and cleanup operations.
#
# Usage: restore.sh <USER_NAMESPACE>
set -Eeuo pipefail
source "../../tests/common/scripts/lib.sh"

user_ns="${1:?user namespace is required}"
state_file="${E2E_STATE_DIR:-/tmp/e2e-ape-state}/gatekeeper.replicas"

replicas="$(cat "${state_file}" 2>/dev/null || true)"
current="$(kubectl -n "${APE_NS}" get deployment "${GATEKEEPER_DEPLOY}" -o jsonpath='{.spec.replicas}')"
if [[ -z "${replicas}" || "${replicas}" == "0" ]]; then
  # Nothing recorded: keep what is there unless the webhook is still scaled to zero.
  replicas="${current}"
  [[ "${replicas}" != "0" ]] || replicas=1
fi

if [[ "${current}" != "${replicas}" ]]; then
  log "Scaling ${GATEKEEPER_DEPLOY} from ${current} back to ${replicas}"
  kubectl -n "${APE_NS}" scale deployment "${GATEKEEPER_DEPLOY}" --replicas="${replicas}" "${AS_DECKHOUSE[@]}"
fi
rm -f "${state_file}"
kubectl -n "${APE_NS}" rollout status deployment "${GATEKEEPER_DEPLOY}" --timeout=7m

deadline=$(( $(date +%s) + 120 ))
until [[ "$(kubectl -n "${APE_NS}" get endpointslices -l kubernetes.io/service-name="${GATEKEEPER_SVC}" -o json \
  | jq '[.items[].endpoints[]? | select(.conditions.ready == true)] | length')" -gt 0 ]]; do
  (( $(date +%s) < deadline )) || fail "${GATEKEEPER_SVC} has no ready endpoints after the restore"
  sleep 2
done

if kubectl get namespace "${user_ns}" >/dev/null 2>&1; then
  bash ../../tests/common/scripts/probe.sh --expect allow --namespace "${user_ns}" --name restored --timeout 120
fi
log "Gatekeeper webhook restored with ${replicas} replicas and admission works"
