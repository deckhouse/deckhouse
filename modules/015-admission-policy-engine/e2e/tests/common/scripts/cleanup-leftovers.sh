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

# Deletes what a killed run of the suite may leave on the cluster: the namespaces `e2e-ape-*`
# created by the tests, the system namespaces `d8-e2e-ape-*` created by d8-namespace.sh, and the
# cluster-scoped objects of the tests and their scripts (SecurityPolicy e2e-ape-sp-signatures of
# verify-image-signatures/create-policy.sh, the other policies, mutators and RBAC objects applied
# from the manifests). The suite namespace itself (`e2e-ape`) is left to the Taskfile.
# The Taskfile runs it before each suite and as a deferred command after it.
set -Eeuo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

# list_names RESOURCE prints the names of the objects of the suite. A resource type that the
# cluster does not serve (a disabled module) has no objects; any other error aborts.
list_names() {
  local out err rc=0
  err="$(mktemp)"
  out="$(kubectl get "$1" -o json 2>"${err}")" || rc=$?
  if (( rc != 0 )); then
    if grep -q "the server doesn't have a resource type" "${err}"; then
      rm -f "${err}"
      return 0
    fi
    >&2 cat "${err}"
    rm -f "${err}"
    return "${rc}"
  fi
  rm -f "${err}"
  jq -r '.items[].metadata.name | select(startswith("e2e-ape-") or startswith("d8-system-default-e2e-ape-"))' <<<"${out}"
}

# Gatekeeper objects go first, while their namespaces and the webhook still exist.
for resource in \
    securitypolicies.deckhouse.io \
    operationpolicies.deckhouse.io \
    assign.mutations.gatekeeper.sh \
    assignmetadata.mutations.gatekeeper.sh \
    clusterrolebindings.rbac.authorization.k8s.io \
    clusterroles.rbac.authorization.k8s.io; do
  names="$(list_names "${resource}")"
  for name in ${names}; do
    log "Deleting the leftover ${resource} ${name}"
    kubectl delete "${resource}" "${name}" --ignore-not-found --wait=true --timeout=2m
  done
done

namespaces_json="$(kubectl get namespaces -o json)"
user_namespaces="$(jq -r '.items[].metadata.name | select(startswith("e2e-ape-"))' <<<"${namespaces_json}")"
system_namespaces="$(jq -r '.items[].metadata.name | select(startswith("d8-e2e-ape-"))' <<<"${namespaces_json}")"
if [[ -n "${user_namespaces}" ]]; then
  log "Deleting the leftover namespaces: $(tr '\n' ' ' <<<"${user_namespaces}")"
  # shellcheck disable=SC2086
  bash "${E2E_LIB_DIR:?E2E_LIB_DIR is not set, run the suite through its Taskfile}/delete-namespace.sh" ${user_namespaces}
fi
for ns in ${system_namespaces}; do
  bash "$(dirname "${BASH_SOURCE[0]}")/d8-namespace.sh" delete "${ns}"
done
