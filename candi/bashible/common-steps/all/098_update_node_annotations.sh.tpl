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
