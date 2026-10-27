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

# Restarts the Gatekeeper webhook with `kubectl rollout restart` while a probe sends a dry-run pod
# every few hundred milliseconds, and counts the requests the API server could not hand to the
# webhook.
#
# In high availability mode (two replicas, maxUnavailable 1, maxSurge 0) one replica serves while
# the other restarts, so the expected number of webhook errors is zero. The suite does not switch
# high availability on: the module takes it from the global setting, and the required pod
# anti-affinity would leave the second replica Pending on a cluster with a single eligible node.
# On a single replica the script reports the count and checks only that admission works after the
# restart.
#
# The waits below add up to about 15 minutes at most, inside the 16m timeout of the Chainsaw step.
# Chainsaw kills the script with SIGKILL on a timeout and the EXIT trap does not run, so the probe
# loop also stops on its own deadline and when the script is gone.
#
# Usage: rollout-under-probe.sh <NAMESPACE>
set -Eeuo pipefail
source "../../tests/common/scripts/lib.sh"

ns="${1:?namespace is required}"
workdir="$(mktemp -d)"
trap 'touch "${workdir}/stop"; wait 2>/dev/null || true; rm -rf "${workdir}"' EXIT

replicas="$(kubectl -n "${APE_NS}" get deployment "${GATEKEEPER_DEPLOY}" -o jsonpath='{.spec.replicas}')"
ready="$(kubectl -n "${APE_NS}" get deployment "${GATEKEEPER_DEPLOY}" -o jsonpath='{.status.readyReplicas}')"
ha=false
if (( replicas >= 2 )); then
  ha=true
fi
log "${GATEKEEPER_DEPLOY}: ${replicas} replicas, ${ready:-0} ready, high availability: ${ha}"

bash ../../tests/common/scripts/probe.sh --expect allow --namespace "${ns}" --name before --timeout 90 >/dev/null
old_pods="$(kubectl -n "${APE_NS}" get pods -l app=gatekeeper,control-plane=controller-manager -o jsonpath='{.items[*].metadata.name}')"

object="$(compliant_pod e2e-ape-rollout-probe "${ns}")"
probe_loop() {
  local rc out parent="$$" deadline=$(( $(date +%s) + 900 ))
  while [[ ! -f "${workdir}/stop" ]] && kill -0 "${parent}" 2>/dev/null && (( $(date +%s) < deadline )); do
    rc=0
    out="$(kubectl create --dry-run=server -o name --request-timeout=20s -f - <<<"${object}" 2>&1)" || rc=$?
    if (( rc == 0 )); then
      echo "ok" >> "${workdir}/results"
    elif grep -qE "failed calling webhook|context deadline exceeded|connection refused|no endpoints available|Timeout" <<<"${out}"; then
      echo "webhook-error $(date -u +%H:%M:%S) ${out}" >> "${workdir}/results"
    else
      echo "other-error $(date -u +%H:%M:%S) ${out}" >> "${workdir}/results"
    fi
    sleep 0.2
  done
}
touch "${workdir}/results"
# Detached from the step output, so an orphaned loop cannot hold the pipes of Chainsaw.
probe_loop </dev/null >/dev/null 2>&1 &

kubectl -n "${APE_NS}" rollout restart deployment "${GATEKEEPER_DEPLOY}" "${AS_DECKHOUSE[@]}"
kubectl -n "${APE_NS}" rollout status deployment "${GATEKEEPER_DEPLOY}" --timeout=8m
# The rollout reports success while the old pods may still be terminating (grace period 60s) and
# still listed as endpoints. The probe keeps running until every old pod is gone, and for a short
# window after that for the endpoint change to reach the API server. All old pods share one 180s
# deadline.
old_pods_deadline=$(( $(date +%s) + 180 ))
for pod in ${old_pods}; do
  left=$(( old_pods_deadline - $(date +%s) ))
  (( left > 0 )) || left=1
  kubectl -n "${APE_NS}" wait "pod/${pod}" --for=delete --timeout="${left}s" \
    || fail "old pod ${pod} was not deleted within 180s after the rollout"
done
sleep 10
touch "${workdir}/stop"
wait

total="$(wc -l < "${workdir}/results")"
ok="$(grep -c '^ok$' "${workdir}/results" || true)"
webhook_errors="$(grep -c '^webhook-error' "${workdir}/results" || true)"
other_errors="$(grep -c '^other-error' "${workdir}/results" || true)"
log "Probe requests: ${total}, admitted: ${ok}, webhook errors: ${webhook_errors}, other errors: ${other_errors}"
grep -v '^ok$' "${workdir}/results" | head -20 || true

new_pods="$(kubectl -n "${APE_NS}" get pods -l app=gatekeeper,control-plane=controller-manager -o json \
  | jq -r '[.items[] | select(.metadata.deletionTimestamp == null) | .metadata.name] | join(" ")')"
for pod in ${old_pods}; do
  if grep -qw "${pod}" <<<"${new_pods}"; then
    fail "pod ${pod} survived the rollout restart"
  fi
done
log "Every webhook pod was replaced: ${old_pods} -> ${new_pods}"

(( total >= 10 )) || fail "the probe sent only ${total} requests, the measurement is not meaningful"
(( other_errors == 0 )) || fail "the probe got ${other_errors} rejections not caused by the webhook availability"
if [[ "${ha}" == "true" ]]; then
  (( webhook_errors == 0 )) || fail "${webhook_errors} requests failed to reach the webhook during the rollout in high availability mode"
  log "Zero webhook errors during the rollout in high availability mode"
else
  log "NOT ASSERTED: the webhook runs a single replica, ${webhook_errors} webhook errors are reported, zero is required only in high availability mode"
fi

bash ../../tests/common/scripts/probe.sh --expect allow --namespace "${ns}" --name after --timeout 90 >/dev/null
log "Admission works after the rollout"
