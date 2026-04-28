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

# $1 annotation
function add_node_annotation() {
  local annotation="$1"
  local failure_count=0
  local failure_limit=5

  until bb-curl-helper-patch-node-metadata "$(bb-d8-node-name)" "annotations" "${annotation}"; do
    failure_count=$((failure_count + 1))
    if [[ $failure_count -eq $failure_limit ]]; then
      bb-log-error "ERROR: Failed to annotate node $(bb-d8-node-name)"
      exit 1
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

  until bb-curl-helper-patch-node-metadata "$(bb-d8-node-name)" "annotations" "${annotation}-"; do
    failure_count=$((failure_count + 1))
    if [[ $failure_count -eq $failure_limit ]]; then
      bb-log-error "ERROR: Failed to annotate node $(bb-d8-node-name)"
      exit 1
    fi
    bb-log-error "failed to annotate node $(bb-d8-node-name)"
    sleep 10
  done
}

{{- if hasKey .nodeGroup "staticInstances" }}
if [[ -f /var/lib/bashible/node-spec-provider-id ]]; then
  PROVIDER_ID="$( bb-curl-kube "/api/v1/nodes/$(bb-d8-node-name)" | jq -r '.spec.providerID' )"

  if [[ "${PROVIDER_ID}" == "static://" ]]; then
    add_node_annotation node.deckhouse.io/provider-id="$(cat /var/lib/bashible/node-spec-provider-id)"
  fi
fi
{{- end }}

{{/*
  This annotation is required by the registry module to track which 
  version of the registry configuration is currently applied on the node.
*/}}
add_node_annotation registry.deckhouse.io/version={{ .registry.version | quote }}

# check if d8-dhctl-converger user exists and annotate node
{{- if eq .nodeGroup.name "master" }} 
converger_user_annotation="$(bb-curl-kube "/api/v1/nodes/$D8_NODE_HOSTNAME" | jq -r '.metadata.annotations."node.deckhouse.io/has-converger-nodeuser"')"
if grep -qP "^d8-dhctl-converger" /etc/passwd; then
  converger_user_exists="1"
  else
    converger_user_exists="0"
fi

if [[ "$converger_user_annotation" != "null" && "$converger_user_exists" == "0" ]]; then
  remove_node_annotation "node.deckhouse.io/has-converger-nodeuser"
else if [[ "$converger_user_annotation" == "null" && "$converger_user_exists" == "1" ]]; then
  add_node_annotation "node.deckhouse.io/has-converger-nodeuser=true"
  fi
fi
{{- end }}
