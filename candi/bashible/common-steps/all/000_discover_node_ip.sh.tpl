# Copyright 2021 Flant JSC
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

{{- $candi := "candi/bashible/bb_node_ip.sh.tpl" -}}
{{- $deckhouse := "/deckhouse/candi/bashible/bb_node_ip.sh.tpl" -}}
{{- $bbni := .Files.Get $deckhouse | default (.Files.Get $candi) -}}
{{- tpl $bbni . }}

{{- if or (eq .runType "ClusterBootstrap") (eq .nodeGroup.nodeType "Static") }}
if [ -z "$(cat /var/lib/bashible/discovered-node-ip)" ] ; then
  bb-log-error "Failed to discover node_ip that matches internalNetworkCIDRs."
  exit 1
fi
{{- end }}
