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

# bashible: parallel-group=light-labels

{{- $nodeTypeList := list "CloudEphemeral" "CloudPermanent" "CloudStatic" }}
mkdir -p /var/lib/node_labels/
{{- if and (has .nodeGroup.nodeType $nodeTypeList) .nodeGroup.cloudProviderType }}
echo "node.deckhouse.io/cloud-provider-type={{ .nodeGroup.cloudProviderType }}" > /var/lib/node_labels/cloud-provider-type
{{- else }}
rm -f /var/lib/node_labels/cloud-provider-type
{{- end }}
