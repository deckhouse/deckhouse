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

bb-kubectl() {
  kubectl --request-timeout 60s ${@}
}

bb-deckhouse-get-disruptive-update-approval() {
    if [ "$FIRST_BASHIBLE_RUN" == "yes" ]; then
        return 0
    fi

    if bb-flag? disruption; then
      return 0
    fi

    attempt=0
    until
        node_data="$(
          bb-kubectl --kubeconfig=/etc/kubernetes/kubelet.conf get node "${D8_NODE_HOSTNAME}" -o json | jq '
          {
            "resourceVersion": .metadata.resourceVersion,
            "isDisruptionApproved": (.metadata.annotations | has("update.node.deckhouse.io/disruption-approved")),
            "isDisruptionRequired": (.metadata.annotations | has("update.node.deckhouse.io/disruption-required"))
          }
        ')" &&
         jq -ne --argjson n "$node_data" '(($n.isDisruptionApproved | not) and ($n.isDisruptionRequired)) or ($n.isDisruptionApproved)' >/dev/null
    do
        attempt=$(( attempt + 1 ))
        if [ -n "${MAX_RETRIES-}" ] && [ "$attempt" -gt "${MAX_RETRIES}" ]; then
            bb-log-error "ERROR: Failed to annotate Node with annotation 'update.node.deckhouse.io/disruption-required='."
            exit 1
        fi
        if bb-flag? rolling-update; then
          bb-log-info "Annotating Node with annotation 'update.node.deckhouse.io/rolling-update='."
          bb-log-info "The node will be deleted and a new one will be created."
          bb-kubectl \
            --kubeconfig=/etc/kubernetes/kubelet.conf \
            --resource-version="$(jq -nr --argjson n "$node_data" '$n.resourceVersion')" \
            annotate node "${D8_NODE_HOSTNAME}" update.node.deckhouse.io/rolling-update= || { bb-log-info "Retry setting update.node.deckhouse.io/rolling-update= annotation on Node in 10 sec..."; sleep 10; }
          exit 0
        else
          bb-log-info "Disruption required, asking for approval."
          bb-log-info "Annotating Node with annotation 'update.node.deckhouse.io/disruption-required='."
          bb-kubectl \
            --kubeconfig=/etc/kubernetes/kubelet.conf \
            --resource-version="$(jq -nr --argjson n "$node_data" '$n.resourceVersion')" \
            annotate node "${D8_NODE_HOSTNAME}" update.node.deckhouse.io/disruption-required= || { bb-log-info "Retry setting update.node.deckhouse.io/disruption-required= annotation on Node in 10 sec..."; sleep 10; }
        fi
    done

    bb-log-info "Disruption required, waiting for approval"

    attempt=0
    until
      bb-kubectl --kubeconfig=/etc/kubernetes/kubelet.conf get node "${D8_NODE_HOSTNAME}" -o json | \
      jq -e '.metadata.annotations | has("update.node.deckhouse.io/disruption-approved")' >/dev/null
    do
        attempt=$(( attempt + 1 ))
        if [ -n "${MAX_RETRIES-}" ] && [ "$attempt" -gt "${MAX_RETRIES}" ]; then
            bb-log-error "ERROR: Failed to get annotation 'update.node.deckhouse.io/disruption-approved' from Node."
            exit 1
        fi
        bb-log-info "Step needs to make some disruptive action. It will continue upon approval:"
        bb-log-info "kubectl annotate node ${D8_NODE_HOSTNAME} update.node.deckhouse.io/disruption-approved="
        bb-log-info "Retry in 10sec..."
        sleep 10
    done

    bb-log-info "Disruption approved!"
    bb-flag-set disruption
}
