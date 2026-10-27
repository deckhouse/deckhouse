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

# Turns the multitenancy mode of user-authz on for a test and back off after it.
#
# Usage: multitenancy.sh enable|restore
#
# enable  - records the original spec of ModuleConfig user-authz in
#           $E2E_STATE_DIR (once, so a second call does not overwrite it),
#           merge-patches settings.enableMultiTenancy=true (or creates the
#           ModuleConfig) and waits until the whole chain is live: the
#           d8-user-authz-multitenancy-state ConfigMap, the authorization webhook
#           DaemonSet, permission-browser-apiserver and its APIService, and every
#           kube-apiserver Pod started with --authorization-config (the flag
#           control-plane-manager adds when user-authz passes it the webhook).
#           If the mode is already on, the ModuleConfig is left untouched and
#           only the waits run.
# restore - puts the recorded spec back (or deletes the ModuleConfig the suite
#           created) and waits until kube-apiserver runs without the webhook
#           again, so that the next suite starts on a settled control plane.
#           The multitenancy ModuleConfig webhook refuses to switch the mode off
#           while a ClusterAuthorizationRule still uses limitNamespaces,
#           namespaceSelector or allowAccessToSystemNamespaces, so the call is
#           retried while the test rules are being deleted.
#
# Switching the mode restarts kube-apiserver; on a single-master cluster the API
# is unreachable for a while, which is why every wait tolerates errors.
#
# The callers run each action in a step with a 40-minute timeout, and Chainsaw kills the script
# with SIGKILL when it is reached. All waits of one action therefore share a budget of
# MT_BUDGET_SECONDS (2100 by default): every wait is cut to what is left of it, so that a failed
# wait still prints webhook-evidence.sh and exits before the step timeout.
set -Eeuo pipefail

# shellcheck source=lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

action="${1:?action enable|restore is required}"
state_file="${E2E_STATE_DIR}/user-authz-moduleconfig.json"
mkdir -p "${E2E_STATE_DIR}"
scripts_dir="$(dirname "${BASH_SOURCE[0]}")"

budget_deadline=$(( $(date +%s) + ${MT_BUDGET_SECONDS:-2100} ))

# capped <SECONDS> prints SECONDS cut to what is left of the budget, at least 1.
capped() {
  local left=$(( budget_deadline - $(date +%s) ))
  (( left >= 1 )) || left=1
  (( $1 < left )) && echo "$1" || echo "${left}"
}

# budget_wait <SECONDS> <DESCRIPTION> <COMMAND...> is wait_for with the timeout cut to the budget.
budget_wait() {
  local timeout
  timeout="$(capped "$1")"
  shift
  wait_for "${timeout}" "$@"
}

# fail_with_evidence <REASON> prints the webhook evidence and stops the script.
fail_with_evidence() {
  >&2 log "$1"
  bash "${scripts_dir}/webhook-evidence.sh" "$1"
  exit 1
}

# ensure_user_authz runs ensure-user-authz.sh (its own waits add up to 30 minutes) within the budget.
ensure_user_authz() {
  timeout "$(capped 1800)" bash "${scripts_dir}/ensure-user-authz.sh"
}

mt_state() {
  kubectl -n d8-user-authz get configmap d8-user-authz-multitenancy-state \
    -o jsonpath='{.data.enableMultiTenancy}' 2>/dev/null || true
}

mt_state_is() {
  [[ "$(mt_state)" == "$1" ]]
}

# apiserver_webhook_is true|false: every control-plane node runs a Ready kube-apiserver
# Pod, and the --authorization-config flag is present (true) or absent (false) in all of them.
apiserver_webhook_is() {
  local want="$1" masters pods
  masters="$(kubectl get nodes -l node-role.kubernetes.io/control-plane -o name 2>/dev/null | wc -l)" || return 1
  (( masters > 0 )) || return 1
  pods="$(kubectl -n kube-system get pods -l component=kube-apiserver -o json 2>/dev/null)" || return 1
  jq -e --arg want "${want}" --argjson masters "${masters}" '
    .items as $pods
    | ($pods | length) == $masters
      and all($pods[];
        ((.status.conditions // []) | any(.type == "Ready" and .status == "True"))
        and (
          [.spec.containers[] | select(.name == "kube-apiserver") | ((.command // []) + (.args // []))[]]
          | any(startswith("--authorization-config="))
        ) == ($want == "true"))
  ' <<<"${pods}" >/dev/null
}

# webhook_reachable_steadily: six reviews in a row, 5 seconds apart, are evaluated without an
# evaluationError. The user has no rule, so the webhook has no opinion and RBAC answers; an
# unreachable webhook shows up as an evaluationError (failurePolicy Deny). The unique names
# bypass the authorizer cache of kube-apiserver. The Pod spec alone is not enough: the first
# run showed connection refused on 127.0.0.1:40443 a minute after every Pod looked ready.
# The first failure and the recovery are recorded with webhook-evidence.sh, to tell an eviction
# of the webhook Pod (for example by the VPA updater) from a slow start.
webhook_unreachable_seen=""

webhook_reachable_steadily() {
  local result _
  for _ in 1 2 3 4 5 6; do
    result="$(sar_decide e2e-authz-reachability get "" pods "" default "e2e-authz-reach-$(date +%s%N)")"
    if [[ "${result%% *}" == "error" ]]; then
      log "Webhook not reachable yet: ${result}"
      if [[ -z "${webhook_unreachable_seen}" ]]; then
        webhook_unreachable_seen="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
        bash "${scripts_dir}/webhook-evidence.sh" "first unreachable review"
      fi
      return 1
    fi
    sleep 5
  done
  if [[ -n "${webhook_unreachable_seen}" ]]; then
    log "Webhook reachable again; it was first unreachable at ${webhook_unreachable_seen}"
    bash "${scripts_dir}/webhook-evidence.sh" "reachable again"
  fi
}

apiservice_available() {
  [[ "$(kubectl get apiservice v1alpha1.authorization.deckhouse.io \
    -o jsonpath='{.status.conditions[?(@.type=="Available")].status}' 2>/dev/null)" == "True" ]]
}

module_ready() {
  [[ "$(kubectl get module user-authz -o jsonpath='{.status.phase}' 2>/dev/null)" == "Ready" ]]
}

rollout_done() {
  kubectl -n d8-user-authz rollout status "$1" --timeout=30s >/dev/null 2>&1
}

enable() {
  if mt_state_is true; then
    log "Multitenancy is already enabled in the cluster; ModuleConfig user-authz is left as is"
  else
    local mc
    # Any API error stops the script: a failed read must never be taken for an absent ModuleConfig,
    # which restore would then delete.
    mc="$(kubectl get moduleconfig user-authz --ignore-not-found -o json)"
    if [[ -n "${mc}" ]]; then
      if jq -e '.spec.settings.controlPlaneConfigurator.enabled == false' <<<"${mc}" >/dev/null; then
        >&2 log "settings.controlPlaneConfigurator.enabled is false: kube-apiserver would not be pointed at the webhook, the multitenancy cases cannot run"
        exit 1
      fi
      [[ -f "${state_file}" ]] || jq -c '.spec' <<<"${mc}" > "${state_file}"
      local patch='{"spec":{"settings":{"enableMultiTenancy":true}}}'
      if jq -e '.spec.version == null' <<<"${mc}" >/dev/null; then
        patch='{"spec":{"version":1,"settings":{"enableMultiTenancy":true}}}'
      fi
      kubectl patch moduleconfig user-authz --type merge -p "${patch}"
    else
      [[ -f "${state_file}" ]] || echo '"ABSENT"' > "${state_file}"
      kubectl apply -f - <<'MC'
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authz
spec:
  version: 1
  settings:
    enableMultiTenancy: true
MC
    fi
    log "Requested enableMultiTenancy=true, original spec recorded in ${state_file}"
  fi

  budget_wait 900 "ConfigMap d8-user-authz-multitenancy-state says true" mt_state_is true \
    || fail_with_evidence "the multitenancy state ConfigMap did not switch to true"
  budget_wait 900 "module user-authz is Ready" module_ready \
    || fail_with_evidence "module user-authz is not Ready"
  budget_wait 600 "DaemonSet user-authz-webhook rolled out" rollout_done daemonset/user-authz-webhook \
    || fail_with_evidence "DaemonSet user-authz-webhook did not roll out"
  budget_wait 600 "Deployment permission-browser-apiserver rolled out" rollout_done deployment/permission-browser-apiserver \
    || fail_with_evidence "Deployment permission-browser-apiserver did not roll out"
  budget_wait 600 "APIService v1alpha1.authorization.deckhouse.io is Available" apiservice_available \
    || fail_with_evidence "APIService v1alpha1.authorization.deckhouse.io is not Available"
  budget_wait 1500 "every kube-apiserver runs with the authorization webhook" apiserver_webhook_is true \
    || fail_with_evidence "kube-apiserver does not run with the authorization webhook"
  budget_wait 600 "kube-apiserver reaches the webhook steadily" webhook_reachable_steadily \
    || fail_with_evidence "the webhook is still unreachable"
  bash "${E2E_LIB_DIR}/wait-deckhouse-queue.sh" "$(capped 900)" \
    || fail_with_evidence "the Deckhouse queue is not empty"
  ensure_user_authz || fail_with_evidence "user-authz-controller does not reconcile"
}

restore_once() {
  local orig="$1"
  if [[ "${orig}" == '"ABSENT"' ]]; then
    kubectl delete moduleconfig user-authz --ignore-not-found
  else
    kubectl patch moduleconfig user-authz --type json \
      -p "[{\"op\":\"replace\",\"path\":\"/spec\",\"value\":${orig}}]"
  fi
}

restore() {
  if [[ ! -f "${state_file}" ]]; then
    log "ModuleConfig user-authz was not changed by the suite, nothing to restore"
    return 0
  fi
  local orig
  orig="$(cat "${state_file}")"
  log "Restoring ModuleConfig user-authz spec: ${orig}"
  budget_wait 300 "ModuleConfig user-authz restored" restore_once "${orig}"
  rm -f "${state_file}"

  # The suite only switches the mode on when it was off, so the original state is "off".
  budget_wait 900 "ConfigMap d8-user-authz-multitenancy-state says false" mt_state_is false
  budget_wait 1500 "every kube-apiserver runs without the authorization webhook" apiserver_webhook_is false
  budget_wait 900 "module user-authz is Ready" module_ready
  bash "${E2E_LIB_DIR}/wait-deckhouse-queue.sh" "$(capped 900)"
  # The kube-apiserver restart may have restarted user-authz-controller; wait until it reconciles.
  ensure_user_authz
}

case "${action}" in
  enable) enable ;;
  restore) restore ;;
  *)
    >&2 echo "unknown action '${action}', expected enable or restore"
    exit 1
    ;;
esac
