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

# Shared helpers of the admission-policy-engine e2e scripts. Source the file, do not run it.

E2E_IMAGE="registry.k8s.io/e2e-test-images/agnhost:2.53"
APE_NS="d8-admission-policy-engine"
APE_VWC="d8-admission-policy-engine-config"
GATEKEEPER_DEPLOY="gatekeeper-controller-manager"
GATEKEEPER_AUDIT_DEPLOY="gatekeeper-audit"
GATEKEEPER_SVC="gatekeeper-webhook-service"

# The Deckhouse service account is the only identity the platform lets:
#   - create a namespace named `d8-*` (ValidatingAdmissionPolicy system-ns.deckhouse.io);
#   - set or change the `heritage: deckhouse` label (deny-set-heritage-label.deckhouse.io,
#     label-objects.deckhouse.io);
#   - change a controller whose pod template carries `gatekeeper.sh/operation: webhook`
#     (deny-gatekeeper-webhook-operation-label-controllers), which is the case for Gatekeeper.
# The cluster-admin kubeconfig of the master impersonates it for exactly these operations.
AS_DECKHOUSE=(
  --as=system:serviceaccount:d8-system:deckhouse
  --as-group=system:serviceaccounts
  --as-group=system:serviceaccounts:d8-system
  --as-group=system:authenticated
)

log() {
  echo "[$(date -u +%H:%M:%S)] $*"
}

fail() {
  >&2 echo "[$(date -u +%H:%M:%S)] FAIL: $*"
  exit 1
}

# compliant_pod NAME NAMESPACE
# Prints a Pod that satisfies Pod Security Standard Restricted and every field of the
# SecurityPolicy used by the suite. The tests derive the violating variants from it with jq.
compliant_pod() {
  jq -n --arg name "$1" --arg ns "$2" --arg image "${E2E_IMAGE}" '{
    apiVersion: "v1",
    kind: "Pod",
    metadata: {name: $name, namespace: $ns, labels: {"e2e-ape-probe": "true"}},
    spec: {
      automountServiceAccountToken: false,
      terminationGracePeriodSeconds: 1,
      tolerations: [{operator: "Exists"}],
      securityContext: {
        runAsNonRoot: true,
        runAsUser: 1000,
        runAsGroup: 1000,
        seccompProfile: {type: "RuntimeDefault"}
      },
      containers: [{
        name: "app",
        image: $image,
        args: ["pause"],
        resources: {
          requests: {cpu: "10m", memory: "16Mi"},
          limits: {cpu: "50m", memory: "32Mi"}
        },
        securityContext: {
          allowPrivilegeEscalation: false,
          readOnlyRootFilesystem: true,
          capabilities: {drop: ["ALL"]}
        }
      }]
    }
  }'
}

# as_deployment reads a Pod from stdin and prints a Deployment with the same pod template.
as_deployment() {
  jq '{
    apiVersion: "apps/v1",
    kind: "Deployment",
    metadata: {name: .metadata.name, namespace: .metadata.namespace, labels: .metadata.labels},
    spec: {
      replicas: 1,
      selector: {matchLabels: {"e2e-ape-workload": .metadata.name}},
      template: {
        metadata: {labels: ((.metadata.labels // {}) + {"e2e-ape-workload": .metadata.name})},
        spec: .spec
      }
    }
  }'
}

# wait_default_sa NAMESPACE waits for the ServiceAccount admission plugin to have its account,
# without which every Pod create in a fresh namespace is rejected for an unrelated reason.
wait_default_sa() {
  local ns="$1" deadline=$(( $(date +%s) + 120 ))
  until kubectl -n "${ns}" get serviceaccount default >/dev/null 2>&1; do
    (( $(date +%s) < deadline )) || fail "ServiceAccount default did not appear in ${ns}"
    sleep 2
  done
}

# wait_deckhouse_queue waits until the Deckhouse main queue is empty twice in a row, so that a
# ModuleConfig change has been rendered. See testing/e2e/lib/wait-deckhouse-queue.sh.
wait_deckhouse_queue() {
  bash "${E2E_LIB_DIR:?E2E_LIB_DIR is not set, run the suite through its Taskfile}/wait-deckhouse-queue.sh" 600
}
