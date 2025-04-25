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
label_path="/var/lib/node_labels/containerd-conf"

# Check additional configs containerd
if ls /etc/containerd/conf.d/*.toml >/dev/null 2>/dev/null; then
  label_value="custom"
else
  label_value="default"
fi

echo "node.deckhouse.io/containerd-config=${label_value}" > "${label_path}"
  {{- else -}}
rm -f "${label_path}"
  {{- end  }}
{{- end  }}
