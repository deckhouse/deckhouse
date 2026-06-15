# Copyright 2023 Flant JSC
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

{{ if eq .runType "Normal" }}
if [ "$FIRST_BASHIBLE_RUN" == "no" ]; then
  WAITING_APPROVAL_MAX_RETRIES=30
  WAITING_APPROVAL_RETRY_SLEEP_SEC=15

  bb-log-info "Setting update.node.deckhouse.io/waiting-for-approval= annotation on this node"
  attempt=0
  until
    node_data="$(
      bb-curl-kube "/api/v1/nodes/$(bb-d8-node-name)" | jq '
      {
        "resourceVersion": .metadata.resourceVersion,
        "isApproved": (.metadata.annotations | has("update.node.deckhouse.io/approved")),
        "isWaitingForApproval": (.metadata.annotations | has("update.node.deckhouse.io/waiting-for-approval"))
      }
    ')" &&
     jq -ne --argjson n "$node_data" '(($n.isApproved | not) and ($n.isWaitingForApproval)) or ($n.isApproved)' >/dev/null
  do
    attempt=$(( attempt + 1 ))
    if [ "$attempt" -gt "${WAITING_APPROVAL_MAX_RETRIES}" ]; then
      bb-log-error "Failed to set update.node.deckhouse.io/waiting-for-approval= annotation on this node"
      exit 1
    fi
    bb-curl-helper-patch-node-metadata "$(bb-d8-node-name)" "annotations" \
      "--resource-version=$(jq -nr --argjson n "$node_data" '$n.resourceVersion')" \
      "update.node.deckhouse.io/waiting-for-approval=" "node.deckhouse.io/configuration-checksum-" \
      || { bb-log-info "Failed to set the waiting-for-approval annotation, retrying in ${WAITING_APPROVAL_RETRY_SLEEP_SEC} seconds"; sleep "${WAITING_APPROVAL_RETRY_SLEEP_SEC}"; }
  done

  bb-log-info "Waiting for update.node.deckhouse.io/approved= annotation on this node"
  approval_command="kubectl annotate node $(bb-d8-node-name) update.node.deckhouse.io/approved="
  waiting_approval_message="Steps are waiting for approval to start. Deckhouse is performing a rolling update. To force the update, run: ${approval_command}"
  waiting_approval_status_set="no"
  attempt=0
  until
    bb-curl-kube "/api/v1/nodes/$(bb-d8-node-name)" | \
    jq -e '.metadata.annotations | has("update.node.deckhouse.io/approved")' >/dev/null
  do
    if [ "$waiting_approval_status_set" != "yes" ]; then
      bb-waiting-approval-required "${waiting_approval_message}"
      waiting_approval_status_set="yes"
    fi
    attempt=$(( attempt + 1 ))
    if [ "$attempt" -gt "${WAITING_APPROVAL_MAX_RETRIES}" ]; then
      bb-waiting-approval-timeout "Waiting for approval timed out. ${waiting_approval_message}"
      bb-log-info "Approval wait exceeded retry limit (${WAITING_APPROVAL_MAX_RETRIES}), still waiting"
      attempt=0
    fi
    bb-log-info "Waiting for approval to start the next steps. Deckhouse is performing a rolling update."
    bb-log-info "To force the update, run: ${approval_command}"
    sleep "${WAITING_APPROVAL_RETRY_SLEEP_SEC}"
  done
  bb-waiting-approval-not-required
fi
{{ end }}
