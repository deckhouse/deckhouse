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

# Description:
#   Checks whether a containerd TOML configuration file contains custom registry sections:
#   - plugins."io.containerd.grpc.v1.cri".registry.mirrors
#   - plugins."io.containerd.grpc.v1.cri".registry.configs
#
# Input:
#   $1: Path to the containerd configuration file (TOML format)
#
# Output:
#   0: A registry configuration exists
#   1: No registry configuration found
#   >1: Parsing failed
#
# Example input (TOML format):
#   [plugins]
#     [plugins."io.containerd.grpc.v1.cri"]
#       [plugins."io.containerd.grpc.v1.cri".registry]
#         [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
#           [plugins."io.containerd.grpc.v1.cri".registry.mirrors."my.registry"]
#             endpoint = ["http://my.registry"]
#         [plugins."io.containerd.grpc.v1.cri".registry.configs]
#           [plugins."io.containerd.grpc.v1.cri".registry.configs."my.registry".auth]
#             auth = "token"
#           [plugins."io.containerd.grpc.v1.cri".registry.configs."my.registry".tls]
#             insecure_skip_verify = true
#
function contains_custom_registry() {
  local config_path="$1"
  local has_custom_registry

  if ! has_custom_registry=$(/opt/deckhouse/bin/yq -ptoml -oy \
    '.plugins["io.containerd.grpc.v1.cri"].registry | has("mirrors") or has("configs")' \
    "$config_path" 2>/dev/null); then
    >&2 echo "ERROR: Failed to parse TOML config: $config_path"
    exit 1
  fi

  echo "$has_custom_registry" | grep -q "true"
}

mkdir -p /var/lib/node_labels

config_label_path="/var/lib/node_labels/containerd-conf"
registry_label_path="/var/lib/node_labels/containerd-conf-registry"
config_label_value="default"
registry_label_value="default"

# Check additional configs containerd
if ls /etc/containerd/conf.d/*.toml >/dev/null 2>/dev/null; then
  config_label_value="custom"

  # Check each additional config file for a registry block
  for config_file in /etc/containerd/conf.d/*.toml; do
    if contains_custom_registry "${config_file}"; then
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
