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

# Shared helpers of the user-authz e2e scripts. Source it, do not run it.
#
# Access checks use SubjectAccessReview objects created by the cluster-admin
# kubeconfig instead of `kubectl auth can-i --as`. The review is evaluated by
# the same authorizer chain (Node, user-authz webhook, RBAC), but it lets the
# test set every attribute explicitly: an empty namespace for cluster-scoped
# checks, a unique name to bypass the authorizer cache of kube-apiserver, and
# the denied flag that tells an explicit deny of the user-authz webhook from
# the "no opinion" of RBAC.

E2E_STATE_DIR="${E2E_STATE_DIR:-/tmp/e2e-authz-state}"
export E2E_STATE_DIR

# Every impersonated user of a real cluster carries this group, so the reviews do too.
SAR_GROUPS="${SAR_GROUPS:-system:authenticated}"

log() {
  printf '%s %s\n' "$(date -u +%H:%M:%S)" "$*"
}

# sar_decide USER VERB GROUP RESOURCE SUBRESOURCE NAMESPACE [NAME]
#
# Prints "<decision> <reason>", where decision is one of:
#   yes   - status.allowed is true;
#   deny  - status.denied is true (an explicit deny, the user-authz webhook is the only
#           authorizer of the chain that issues one);
#   no    - neither allowed nor denied (no authorizer has an opinion);
#   error - the review could not be created, or an authorizer failed to evaluate it
#           (status.evaluationError, for example the webhook was unreachable and
#           kube-apiserver denied the request by failurePolicy). A deny caused by an
#           outage must not pass for a deny decided by the webhook.
# Empty arguments are left out of resourceAttributes.
sar_decide() {
  local user="$1" verb="$2" group="$3" resource="$4" sub="$5" ns="$6" name="${7:-}"
  local out
  if ! out="$(jq -n \
      --arg user "${user}" --arg groups "${SAR_GROUPS}" \
      --arg verb "${verb}" --arg group "${group}" --arg resource "${resource}" \
      --arg sub "${sub}" --arg ns "${ns}" --arg name "${name}" '
      {
        apiVersion: "authorization.k8s.io/v1",
        kind: "SubjectAccessReview",
        spec: {
          user: $user,
          groups: ($groups | split(",")),
          resourceAttributes: (
            {verb: $verb, group: $group, resource: $resource, subresource: $sub,
             namespace: $ns, name: $name}
            | with_entries(select(.value != ""))
          )
        }
      }' | kubectl create -f - -o json 2>&1)"; then
    echo "error ${out//$'\n'/ }"
    return 0
  fi
  jq -r '
    (if (.status.evaluationError // "") != "" then "error"
     elif .status.allowed == true then "yes"
     elif .status.denied == true then "deny"
     else "no" end)
    + " " + (.status.reason // "")
    + (if (.status.evaluationError // "") != "" then " evaluationError=" + .status.evaluationError else "" end)
  ' <<<"${out}" 2>/dev/null || echo "error unparsable review: ${out//$'\n'/ }"
}

# sar_matches EXPECT DECISION
#   yes  - the action must be allowed;
#   no   - the action must not be allowed (no opinion or an explicit deny);
#   deny - the action must be denied explicitly by the user-authz webhook.
sar_matches() {
  local expect="$1" decision="$2"
  case "${expect}" in
    yes) [[ "${decision}" == "yes" ]] ;;
    no) [[ "${decision}" == "no" || "${decision}" == "deny" ]] ;;
    deny) [[ "${decision}" == "deny" ]] ;;
    *)
      >&2 echo "unknown expectation '${expect}'"
      return 2
      ;;
  esac
}

# field VALUE: "-" stands for an empty matrix cell, "@ns" for the test namespace.
field() {
  case "$1" in
    -) echo "" ;;
    @ns) echo "${NAMESPACE:?NAMESPACE is not set}" ;;
    *) echo "$1" ;;
  esac
}

# wait_for TIMEOUT_SECONDS DESCRIPTION COMMAND...
# Runs COMMAND every 5 seconds until it succeeds or the timeout expires.
wait_for() {
  local timeout="$1" what="$2"
  shift 2
  local deadline=$(( $(date +%s) + timeout ))
  until "$@"; do
    if (( $(date +%s) >= deadline )); then
      >&2 log "Timed out after ${timeout}s waiting for: ${what}"
      return 1
    fi
    sleep 5
  done
  log "Done: ${what}"
}
