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

{{- if eq .runType "Normal" }}
  {{- if eq .cri "Containerd" }}

mkdir -p /var/lib/node_labels

config_label_path="/var/lib/node_labels/containerd-conf"
registry_label_path="/var/lib/node_labels/containerd-conf-registry"
config_label_value="default"
registry_label_value="default"

# Check additional configs containerd
if ls /etc/containerd/conf.d/*.toml >/dev/null 2>/dev/null; then
  config_label_value="custom"

  # Check each additional config file for a registry block
  for path in /etc/containerd/conf.d/*.toml; do
    if bb-ctrd-has-registry-fields "${path}"; then
      registry_label_value="custom"
      break
    fi
  done
fi

echo "node.deckhouse.io/containerd-config=${config_label_value}" > "${config_label_path}"
echo "node.deckhouse.io/containerd-config-registry=${registry_label_value}" > "${registry_label_path}"
  {{- else -}}
rm -f "${config_label_path}"
rm -f "${registry_label_path}"
  {{- end  }}
{{- end  }}
