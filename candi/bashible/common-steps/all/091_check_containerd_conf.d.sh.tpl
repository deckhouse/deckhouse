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

set_containerd_config_label() {
  local label_name="$1"
  local conf_dir="$2"
  local label_file_name="$3"
  local label_dir_path="/var/lib/node_labels/"
  local full_conf_path="/etc/containerd/${conf_dir}"
  local label_value="default"

  if ls "${full_conf_path}/"*.toml >/dev/null 2>&1; then
    label_value="custom"
  fi

  mkdir -p /var/lib/node_labels/
  echo "node.deckhouse.io/${label_name}=${label_value}" > $label_dir_path/$label_file_name
}

{{- if eq .runType "Normal" }}
  {{- if eq .cri "Containerd" }}
    set_containerd_config_label "containerd-config" "conf.d" "containerd-conf"
    rm -f /var/lib/node_labels/containerd-v2-config
  {{- else if eq .cri "ContainerdV2" }}
    set_containerd_config_label "containerd-v2-config" "conf2.d" "containerd-v2-conf"
    rm -f /var/lib/node_labels/containerd-conf
  {{- else }}
    rm -f /var/lib/node_labels/containerd-conf /var/lib/node_labels/containerd-v2-config
  {{- end }}
{{- end }}