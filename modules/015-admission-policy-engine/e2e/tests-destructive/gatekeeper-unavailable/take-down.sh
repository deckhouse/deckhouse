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

# Scales the Gatekeeper webhook to zero and checks what the webhooks do without it:
#   - a pod in a user namespace is rejected with `failed calling webhook` (failurePolicy: Fail);
#   - a pod in d8-system and kube-system is admitted (the system webhooks use failurePolicy: Ignore);
#   - exec into a pod of a system namespace is rejected (deny-exec-heritage keeps failurePolicy: Fail).
#
# Deckhouse renders replicas into the Deployment and may put them back on the next converge of the
# module. The checks take seconds, and the script verifies after them that the Deployment was still
# scaled to zero; if Deckhouse reverted it in between, the attempt is repeated (three at most).
#
# Usage: take-down.sh <USER_NAMESPACE>
set -Eeuo pipefail
source "../../tests/common/scripts/lib.sh"

user_ns="${1:?user namespace is required}"
state_dir="${E2E_STATE_DIR:-/tmp/e2e-ape-state}"
state_file="${state_dir}/gatekeeper.replicas"
probe="../../tests/common/scripts/probe.sh"
mkdir -p "${state_dir}"

if [[ ! -f "${state_file}" ]]; then
  kubectl -n "${APE_NS}" get deployment "${GATEKEEPER_DEPLOY}" -o jsonpath='{.spec.replicas}' > "${state_file}"
fi
log "Original replicas of ${GATEKEEPER_DEPLOY}: $(cat "${state_file}")"

exec_webhook=false
if webhook_present "deny-exec-heritage.admission-policy-engine.deckhouse.io"; then
  exec_webhook=true
else
  # Absence is correct only for a build whose version is `dev`, as in check-exec-heritage.sh.
  # The version is read while Gatekeeper still runs: exec into d8-system is admitted then.
  version="$(kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- cat /deckhouse/version 2>&1 | tr -d '[:space:]' || true)"
  [[ "${version}" == "dev" ]] \
    || fail "the deny-exec-heritage webhook is not rendered while the Deckhouse version is '${version}', not 'dev'"
fi

wait_down() {
  # 90 seconds covers the 60-second termination grace period of the webhook pods. Three attempts of
  # this wait plus the probes (about 100 seconds) stay within the 12-minute step timeout, so the
  # explicit failure message below is always printed.
  local deadline=$(( $(date +%s) + 90 )) pods addrs
  while true; do
    pods="$(kubectl -n "${APE_NS}" get pods -l app=gatekeeper,control-plane=controller-manager -o name | wc -l)"
    addrs="$(kubectl -n "${APE_NS}" get endpointslices -l kubernetes.io/service-name="${GATEKEEPER_SVC}" -o json \
      | jq '[.items[].endpoints[]? | select(.conditions.ready == true)] | length')"
    (( pods == 0 && addrs == 0 )) && return 0
    (( $(date +%s) < deadline )) || return 1
    sleep 2
  done
}

check_outage() {
  bash "${probe}" --expect webhook-error --namespace "${user_ns}" --name user-pod --timeout 30 \
    --match 'admission-policy-engine\.deckhouse\.io' || return 1
  bash "${probe}" --expect allow --namespace d8-system --name e2e-ape-system-pod --timeout 30 || return 1
  bash "${probe}" --expect allow --namespace kube-system --name e2e-ape-system-pod --timeout 30 || return 1
  if [[ "${exec_webhook}" == "true" ]]; then
    local pod out
    pod="$(kubectl -n d8-system get pod -l app=deckhouse,leader=true -o jsonpath='{.items[0].metadata.name}')"
    out="$(kubectl -n d8-system exec "${pod}" -c deckhouse -- true 2>&1 || true)"
    log "exec into d8-system/${pod}: ${out}"
    grep -q 'failed calling webhook "deny-exec-heritage.admission-policy-engine.deckhouse.io"' <<<"${out}" || return 1
  else
    log "NOT APPLICABLE: the deny-exec-heritage webhook is not rendered (Deckhouse version 'dev'), exec is not checked"
  fi
}

for attempt in 1 2 3; do
  log "Attempt ${attempt}: scaling ${GATEKEEPER_DEPLOY} to 0"
  kubectl -n "${APE_NS}" scale deployment "${GATEKEEPER_DEPLOY}" --replicas=0 "${AS_DECKHOUSE[@]}"
  wait_down || fail "Gatekeeper webhook pods or endpoints are still present 90s after scaling to 0"
  log "Gatekeeper webhook has no pods and no ready endpoints"

  rc=0
  check_outage || rc=$?
  replicas="$(kubectl -n "${APE_NS}" get deployment "${GATEKEEPER_DEPLOY}" -o jsonpath='{.spec.replicas}')"
  if [[ "${replicas}" != "0" ]]; then
    log "Deckhouse restored the Deployment to ${replicas} replicas during the checks, repeating"
    continue
  fi
  (( rc == 0 )) || fail "admission did not behave as expected while Gatekeeper was unavailable"
  log "Outage behaviour confirmed"
  exit 0
done
fail "Deckhouse restored ${GATEKEEPER_DEPLOY} during every attempt, the outage could not be observed"
