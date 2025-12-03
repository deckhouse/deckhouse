# Copyright 2025 Flant JSC
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

#1 annotation
function add_node_annotation() {
  local annotation="$1"
  local failure_count=0
  local failure_limit=5

  until bb-kubectl-exec --kubeconfig=/etc/kubernetes/kubelet.conf annotate node $(bb-d8-node-name) "${annotation}" --overwrite; do
    failure_count=$((failure_count + 1))
    if [[ $failure_count -eq $failure_limit ]]; then
      bb-log-error "ERROR: Failed to annotate node $(bb-d8-node-name)"
      break
    fi
    bb-log-error "failed to annotate node $(bb-d8-node-name)"
    sleep 10
  done
}

# $1 annotation
function remove_node_annotation() {
  local annotation="$1"
  local failure_count=0
  local failure_limit=5

  until bb-kubectl-exec --kubeconfig=/etc/kubernetes/kubelet.conf annotate node $(bb-d8-node-name) "${annotation}"- --overwrite; do
    failure_count=$((failure_count + 1))
    if [[ $failure_count -eq $failure_limit ]]; then
      bb-log-error "ERROR: Failed to annotate node $(bb-d8-node-name)"
      break
    fi
    bb-log-error "failed to annotate node $(bb-d8-node-name)"
    sleep 10
  done
}

{{- if hasKey .nodeGroup "staticInstances" }}
if [[ -f /var/lib/bashible/node-spec-provider-id ]]; then
  PROVIDER_ID="$( bb-kubectl-exec get no $(bb-d8-node-name) -o json | jq -r '.spec.providerID' )"

  if [[ "${PROVIDER_ID}" == "static://" ]]; then
    bb-kubectl-exec annotate node $(bb-d8-node-name) node.deckhouse.io/provider-id="$(cat /var/lib/bashible/node-spec-provider-id)"
  fi
fi
{{- end }}

{{/*
  This annotation is required by the registry module to track which 
  version of the registry configuration is currently applied on the node.
*/}}
bb-kubectl-exec annotate node $(bb-d8-node-name) registry.deckhouse.io/version={{ .registry.version | quote }} --overwrite

# check if d8-dhctl-converger user exists and annotate node

CONVERGER_USER_ANNOTATION=$(bb-kubectl-exec --kubeconfig=/etc/kubernetes/kubelet.conf get no "$D8_NODE_HOSTNAME" -o json |jq -r '.metadata.annotations."node.deckhouse.io/has-converger-nodeuser"')
grep "d8-dhctl-converger" /etc/passwd >/dev/null 2>&1
exit_code=$?


if [[ $CONVERGER_USER_ANNOTATION != "null" ]]
  then
    if [[ $exit_code -eq 1 ]]
      then
        remove_node_annotation "node.deckhouse.io/has-converger-nodeuser"
      fi
    else
     if [[ $exit_code -eq 0 ]]
        then
          add_node_annotation "node.deckhouse.io/has-converger-nodeuser=true"
      fi
fi
