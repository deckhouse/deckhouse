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

# Makes sure the release-1.77 hook-based user-authz module is ready before a test
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

# The release renders rule bindings through a Deckhouse hook and Helm rather than a controller Deployment.
kubectl get crd clusterauthorizationrules.deckhouse.io authorizationrules.deckhouse.io >/dev/null
canary="e2e-authz-canary"
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
canary_ready() {
  kubectl get clusterrolebinding user-authz:e2e-authz-canary:user --ignore-not-found -o name | grep -Fxq 'clusterrolebinding.rbac.authorization.k8s.io/user-authz:e2e-authz-canary:user'
}
if ! wait_for 600 "user-authz hook renders the canary binding" canary_ready; then
  kubectl get clusterauthorizationrules.deckhouse.io "${canary}" -o yaml || true
  kubectl -n d8-system get pods -l app=deckhouse -o wide || true
  exit 1
fi
