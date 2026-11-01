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

# Makes sure the user-authz module and its controller are up before a test
# relies on them.
#
# Usage: ensure-user-authz.sh [--enable]
#
# Without arguments the script only waits for the module to be Ready (the test
# steps call it this way). With --enable (the Taskfile calls it once before a
# suite) testing/e2e/lib/module-enabled.sh enables a module that is not Ready
# and records the original `spec.enabled` in $E2E_STATE_DIR for
# cleanup-leftovers.sh. Other settings of the ModuleConfig are not touched.
set -Eeuo pipefail

# shellcheck source=lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

if [[ "${1:-}" == "--enable" ]]; then
  bash "${E2E_LIB_DIR:?E2E_LIB_DIR is not set, run the suite through its Taskfile}/module-enabled.sh" \
    ensure user-authz "${E2E_STATE_DIR}"
fi

if ! kubectl wait module/user-authz --for=jsonpath='{.status.phase}'=Ready --timeout=15m; then
  >&2 log "Module user-authz is not Ready"
  kubectl get module user-authz -o yaml || true
  kubectl get moduleconfig user-authz -o yaml || true
  exit 1
fi

kubectl -n d8-user-authz rollout status deployment/user-authz-controller --timeout=10m
kubectl get crd clusterauthorizationrules.deckhouse.io authorizationrules.deckhouse.io >/dev/null

# A rolled-out Deployment does not mean a reconciling controller. When kube-apiserver restarts
# (switching the multitenancy mode does that), the controller process can lose its lease and
# restart; the new process then waits up to the 60 s lease of its previous incarnation
# (images/user-authz-controller/src/cmd/main.go, leaseDuration) before it reconciles anything.
# A canary rule proves the controller is leading and writing status before a test relies on it.
canary="e2e-authz-canary"

canary_ready() {
  kubectl get clusterauthorizationrules.deckhouse.io "${canary}" -o json 2>/dev/null | jq -e '
    .status.observedGeneration == .metadata.generation
    and ([.status.conditions[]? | select(.type == "Ready" and .status == "True")] | length) > 0
  ' >/dev/null
}

delete_canary() {
  kubectl delete clusterauthorizationrules.deckhouse.io "${canary}" --ignore-not-found --wait=true --timeout=60s >/dev/null 2>&1 || true
}
trap delete_canary EXIT

kubectl apply -f - <<'CAR'
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: e2e-authz-canary
  labels:
    e2e.deckhouse.io/suite: user-authz
spec:
  accessLevel: User
  subjects:
  - kind: User
    name: e2e-authz-canary
CAR

if ! wait_for 300 "user-authz-controller reconciles (canary rule ${canary} is Ready)" canary_ready; then
  kubectl get clusterauthorizationrules.deckhouse.io "${canary}" -o yaml || true
  kubectl -n d8-user-authz get lease user-authz-controller -o yaml || true
  kubectl -n d8-user-authz get pods -l app=user-authz-controller -o wide || true
  kubectl -n d8-user-authz logs -l app=user-authz-controller --tail=50 --prefix || true
  exit 1
fi
