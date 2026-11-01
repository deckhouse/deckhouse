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

# Takes the authorization webhook away and checks that kube-apiserver fails
# closed for the subjects the webhook decides for, while the identities that the
# AuthorizationConfiguration of control-plane-manager excludes keep working.
#
# Usage: fail-closed.sh run|restore
#
# run     - adds a nodeSelector no node matches to DaemonSet user-authz-webhook,
#           waits until no webhook Pod is left, then expects:
#             * a review for e2e-authz-res-editor (allowed by RBAC) is not allowed;
#             * a review for the deckhouse ServiceAccount is allowed;
#             * the cluster-admin kubeconfig of the runner (kubernetes-admin) works.
#           The DaemonSet is restored on exit, whatever the outcome.
# restore - removes the nodeSelector and waits for the DaemonSet to roll out and
#           for the Editor to be allowed again. Safe to call repeatedly.
set -Eeuo pipefail

# shellcheck source=../../tests/common/scripts/lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/../../tests/common/scripts/lib.sh"

ns="${NAMESPACE:?NAMESPACE is not set}"
key="e2e-authz-unschedulable"

no_webhook_pods() {
  local pods
  # A failed list is not "no pods": only a successful, empty answer counts.
  pods="$(kubectl -n d8-user-authz get pods -l app=user-authz-webhook -o name)" || return 1
  [[ -z "${pods}" ]]
}

editor_review() {
  sar_decide e2e-authz-res-editor get "" pods "" "${ns}" "e2e-authz-failclosed-$(date +%s%N)"
}

# editor_refusal: 0 - kube-apiserver refused because the webhook could not be evaluated
# (an evaluationError, which is what failurePolicy Deny produces) or the webhook denied;
# 1 - the review was answered otherwise (allowed, or no opinion without an error);
# 2 - the review itself could not be created, so it says nothing and is retried.
editor_refusal() {
  local result
  result="$(editor_review)"
  echo "${result}" >&2
  case "${result}" in
    "error "*evaluationError=*) return 0 ;;
    "deny "*) return 0 ;;
    "error "*) return 2 ;;
    *) return 1 ;;
  esac
}

editor_allowed() {
  local result
  result="$(editor_review)"
  echo "${result}" >&2
  [[ "${result%% *}" == "yes" ]]
}

restore() {
  if kubectl -n d8-user-authz get daemonset user-authz-webhook -o json \
      | jq -e --arg key "${key}" '.spec.template.spec.nodeSelector[$key] != null' >/dev/null; then
    kubectl -n d8-user-authz patch daemonset user-authz-webhook --type json \
      -p "[{\"op\":\"remove\",\"path\":\"/spec/template/spec/nodeSelector/${key}\"}]"
  fi
  kubectl -n d8-user-authz rollout status daemonset/user-authz-webhook --timeout=10m
  wait_for 300 "the Editor is allowed again" editor_allowed
}

run() {
  trap 'log "Restoring DaemonSet user-authz-webhook"; restore' EXIT
  wait_for 120 "the Editor is allowed while the webhook is up" editor_allowed

  log "Making DaemonSet user-authz-webhook unschedulable"
  kubectl -n d8-user-authz patch daemonset user-authz-webhook --type merge \
    -p "{\"spec\":{\"template\":{\"spec\":{\"nodeSelector\":{\"${key}\":\"true\"}}}}}"
  wait_for 300 "no user-authz-webhook Pod is left" no_webhook_pods

  # Unique names bypass the authorizer cache, so the first review after the last Pod is
  # gone may still race the endpoint; require three refusals in a row.
  local refused=0 deadline=$(( $(date +%s) + 120 ))
  local rc
  while (( refused < 3 )); do
    rc=0
    editor_refusal || rc=$?
    case "${rc}" in
      0) refused=$(( refused + 1 )) ;;
      2) ;;
      *) refused=0 ;;
    esac
    if (( $(date +%s) >= deadline )); then
      >&2 log "kube-apiserver kept allowing the Editor without the webhook: it does not fail closed"
      exit 1
    fi
    sleep 2
  done
  log "kube-apiserver fails closed for the Editor"

  local deckhouse
  deckhouse="$(SAR_GROUPS="system:serviceaccounts,system:serviceaccounts:d8-system,system:authenticated" \
    sar_decide system:serviceaccount:d8-system:deckhouse get "" pods "" d8-system "e2e-authz-failclosed-sa")"
  log "deckhouse ServiceAccount, get pods in d8-system: ${deckhouse}"
  [[ "${deckhouse%% *}" == "yes" ]] || { >&2 log "the excluded deckhouse ServiceAccount must keep working"; exit 1; }

  kubectl get --raw /readyz >/dev/null
  kubectl -n d8-user-authz get daemonset user-authz-webhook >/dev/null
  log "the cluster-admin kubeconfig keeps working"
}

case "${1:-}" in
  run) run ;;
  restore) restore ;;
  *)
    >&2 echo "usage: fail-closed.sh run|restore"
    exit 1
    ;;
esac
