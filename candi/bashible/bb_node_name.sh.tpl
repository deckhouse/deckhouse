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

{{- define "bb-d8-node-name" -}}
bb-d8-node-name() {
  echo $(</var/lib/bashible/discovered-node-name)
}
{{- end }}

{{- define "bb-d8-node-ip" -}}
bb-d8-node-ip() {
  echo $(</var/lib/bashible/discovered-node-ip)
}
{{- end }}

{{- define "bb-discover-node-name" -}}
bb-discover-node-name() {
  local discovered_name_file="/var/lib/bashible/discovered-node-name"
  local discovered_ip_file="/var/lib/bashible/discovered-node-ip"

  if [ ! -s "$discovered_name_file" ]; then
    if [[ -s "$discovered_ip_file" && -s /etc/kubernetes/kubelet.conf ]]; then
      kubectl get node --kubeconfig /etc/kubernetes/kubelet.conf -o wide \
        | awk '{print $1, $6}' \
        | grep "$(cat "$discovered_ip_file")" \
        | awk '{print $1}' > "$discovered_name_file"
    else
    {{- if and (ne .nodeGroup.nodeType "Static") (ne .nodeGroup.nodeType "CloudStatic") }}
      if [[ "$(hostname)" != "$(hostname -s)" ]]; then
        hostnamectl set-hostname "$(hostname -s)"
      fi
    {{- end }}
      hostname > "$discovered_name_file"
    fi
  fi
}
{{- end }}

