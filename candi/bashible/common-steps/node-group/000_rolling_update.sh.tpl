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

if [ "$FIRST_BASHIBLE_RUN" == "yes" ]; then
    return 0
fi

if bb-flag? disruption; then
  return 0
fi

disruptionsApprovalMode={{ .nodeGroup.disruptions.approvalMode | default "Manual" | quote }}
bb-log-info "Disruptions ApprovalMode: ${disruptionsApprovalMode}"

if [ "$disruptionsApprovalMode" == "RollingUpdate" ]; then
  bb-log-info "Annotating Node with annotation 'update.node.deckhouse.io/rolling-update='."
  bb-kubectl \
    --kubeconfig=/etc/kubernetes/kubelet.conf \
    --resource-version="$(jq -nr --argjson n "$node_data" '$n.resourceVersion')" \
    annotate node "$(hostname -s)" update.node.deckhouse.io/rolling-update= || { bb-log-info "Retry setting update.node.deckhouse.io/rolling-update= annotation on Node in 10 sec..."; sleep 10; }
  exit 0
fi
