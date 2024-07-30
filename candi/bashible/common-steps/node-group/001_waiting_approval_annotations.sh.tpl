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

function kubectl_exec() {
  kubectl --request-timeout 60s --kubeconfig=/etc/kubernetes/kubelet.conf ${@}
}

{{ if eq .runType "Normal" }}
if [ "$FIRST_BASHIBLE_RUN" == "no" ]; then
  >&2 echo "Setting update.node.deckhouse.io/waiting-for-approval= annotation on our Node..."
  attempt=0
  until
    node_data="$(
      kubectl_exec get node "${D8_NODE_HOSTNAME}" -o json | jq '
      {
        "resourceVersion": .metadata.resourceVersion,
        "isApproved": (.metadata.annotations | has("update.node.deckhouse.io/approved")),
        "isWaitingForApproval": (.metadata.annotations | has("update.node.deckhouse.io/waiting-for-approval"))
      }
    ')" &&
     jq -ne --argjson n "$node_data" '(($n.isApproved | not) and ($n.isWaitingForApproval)) or ($n.isApproved)' >/dev/null
  do
    attempt=$(( attempt + 1 ))
    if [ -n "${MAX_RETRIES-}" ] && [ "$attempt" -gt "${MAX_RETRIES}" ]; then
      >&2 echo "ERROR: Can't set update.node.deckhouse.io/waiting-for-approval= annotation on our Node."
      exit 1
    fi
    kubectl_exec annotate node "${D8_NODE_HOSTNAME}" \
      --resource-version="$(jq -nr --argjson n "$node_data" '$n.resourceVersion')" \
      update.node.deckhouse.io/waiting-for-approval= node.deckhouse.io/configuration-checksum- \
      || { echo "Retry setting update.node.deckhouse.io/waiting-for-approval= annotation on our Node in 10sec..."; sleep 10; }
  done

  >&2 echo "Waiting for update.node.deckhouse.io/approved= annotation on our Node..."
  attempt=0
  until
    kubectl_exec get node "${D8_NODE_HOSTNAME}" -o json | \
    jq -e '.metadata.annotations | has("update.node.deckhouse.io/approved")' >/dev/null
  do
    attempt=$(( attempt + 1 ))
    if [ -n "${MAX_RETRIES-}" ] && [ "$attempt" -gt "${MAX_RETRIES}" ]; then
      >&2 echo "ERROR: Can't get annotation 'update.node.deckhouse.io/approved' from our Node."
      exit 1
    fi
    echo "Steps are waiting for approval to start."
    echo "Note: Deckhouse is performing a rolling update. If you want to force an update, use the following command."
    echo "kubectl annotate node ${D8_NODE_HOSTNAME} update.node.deckhouse.io/approved="
    echo "Retry in 10sec..."
    sleep 10
  done
fi
{{ end }}
