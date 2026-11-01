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

# Safety net run by the Taskfile after a suite, even when it fails. It deletes
# the cluster-scoped objects the tests label e2e.deckhouse.io/suite=user-authz
# and the extra namespaces of the tests, puts the user-authz webhook DaemonSet
# back on schedule, starts the user-authz-controller replicas a killed test left
# stopped, restores ModuleConfig user-authz if a test left the
# multitenancy mode switched on, and finally puts spec.enabled back with
# testing/e2e/lib/module-enabled.sh if ensure-user-authz.sh --enable changed
# it. Every section runs even when an earlier one fails; the script fails at
# the end if any of them did. Every action is idempotent, so the script is a
# no-op after a clean run.
set -Euo pipefail

scripts_dir="$(dirname "${BASH_SOURCE[0]}")"
# shellcheck source=lib.sh
source "${scripts_dir}/lib.sh"

rc=0

# The suite namespace and the extra namespaces of the tests. A name counts as a leftover when the
# namespace or a Project of that name (an orphan left by a raced adoption) still exists.
extra_namespaces=(
  e2e-authz
  e2e-authz-rbacv2-other
  e2e-authz-converge-other
  e2e-authz-mt-b
  e2e-authz-resilience-other
)

delete_rules() {
  kubectl delete clusterauthorizationrules.deckhouse.io -l e2e.deckhouse.io/suite=user-authz \
    --ignore-not-found --wait=true
}

delete_cluster_roles() {
  kubectl delete clusterroles -l e2e.deckhouse.io/suite=user-authz --ignore-not-found
}

delete_extra_namespaces() {
  local present=() ns
  for ns in "${extra_namespaces[@]}"; do
    if kubectl get namespace "${ns}" >/dev/null 2>&1 || kubectl get projects.deckhouse.io "${ns}" >/dev/null 2>&1; then
      present+=("${ns}")
    fi
  done
  (( ${#present[@]} == 0 )) && return 0
  bash "${E2E_LIB_DIR}/delete-namespace.sh" "${present[@]}"
}

reschedule_webhook() {
  local ds
  ds="$(kubectl -n d8-user-authz get daemonset user-authz-webhook -o json 2>/dev/null)" || return 0
  jq -e '.spec.template.spec.nodeSelector["e2e-authz-unschedulable"] != null' <<<"${ds}" >/dev/null || return 0
  log "Removing the e2e nodeSelector from DaemonSet user-authz-webhook"
  kubectl -n d8-user-authz patch daemonset user-authz-webhook --type json \
    -p '[{"op":"remove","path":"/spec/template/spec/nodeSelector/e2e-authz-unschedulable"}]' &&
    kubectl -n d8-user-authz rollout status daemonset/user-authz-webhook --timeout=10m
}

restore_controller() {
  bash "${scripts_dir}/controller-replicas.sh" restore
}

restore_multitenancy() {
  bash "${scripts_dir}/multitenancy.sh" restore
}

# Runs last: disabling the module removes every rule binding of the cluster.
restore_module_enabled() {
  bash "${E2E_LIB_DIR}/module-enabled.sh" restore user-authz "${E2E_STATE_DIR}"
}

for section in delete_rules delete_cluster_roles delete_extra_namespaces reschedule_webhook \
    restore_controller restore_multitenancy restore_module_enabled; do
  log "Cleanup: ${section}"
  if ! "${section}"; then
    >&2 log "Cleanup section ${section} failed"
    rc=1
  fi
done

# The state directory is removed only when nothing is left to restore, so that a failed
# restore can be repeated by running this script again.
if (( rc == 0 )); then
  rm -rf "${E2E_STATE_DIR}"
  rm -rf "${TMPDIR:-/tmp}"/e2e-authz-probe.* 2>/dev/null || true
else
  >&2 log "Keeping ${E2E_STATE_DIR} for a repeated cleanup"
fi

exit "${rc}"
