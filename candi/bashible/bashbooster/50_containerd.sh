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

# bb-ctrd-v1-has-registry-fields:
# Check if a containerd TOML configuration file contains the
# registry section: `plugins."io.containerd.grpc.v1.cri".registry`
#
# Arguments:
#   $1 — Path to the containerd configuration file (in TOML format)
#
# Returns:
#   0 — Registry configuration section found
#   1 — Registry section not found
#   >1 — Error parsing the TOML file
#
# Example containerd configuration (TOML format):
#
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
bb-ctrd-v1-has-registry-fields() {
  local path="$1"
  local has_registry
  if ! has_registry=$(/opt/deckhouse/bin/yq -ptoml -oy \
    '.plugins["io.containerd.grpc.v1.cri"] | has("registry")' "$path" 2>/dev/null); then
    >&2 echo "ERROR: Failed to parse TOML config: $path"
    exit 1
  fi
  echo "$has_registry" | grep -q "true"
}

# bb-ctrd-v2-has-registry-fields:
# Check if a containerd TOML configuration contains the
# registry section under: plugins."io.containerd.cri.v1.images".registry
#
# Arguments:
#   $1 — Path to the containerd configuration file (in TOML format)
#
# Returns:
#   0 — Registry configuration section found
#   1 — Registry section not found
#   >1 — Error parsing the TOML file
#
# Example containerd configuration (TOML format):
#
#   [plugins."io.containerd.cri.v1.images".registry]
#     [plugins."io.containerd.cri.v1.images".registry.mirrors]
#       [plugins."io.containerd.cri.v1.images".registry.mirrors."docker.io"]
#         endpoint = ["https://registry-1.docker.io"]
#       [plugins."io.containerd.cri.v1.images".registry.mirrors."gcr.io"]
#         endpoint = ["https://gcr.io"]
#     [plugins."io.containerd.cri.v1.images".registry.configs]
#       [plugins."io.containerd.cri.v1.images".registry.configs."gcr.io".auth]
#         username = "_json_key"
#         password = "..."
bb-ctrd-v2-has-registry-fields() {
  local path="$1"
  local has_registry
  if ! has_registry=$(/opt/deckhouse/bin/yq -ptoml -oy \
    '.plugins["io.containerd.cri.v1.images"] | has("registry")' "$path" 2>/dev/null); then
    >&2 echo "ERROR: Failed to parse TOML config: $path"
    exit 1
  fi
  echo "$has_registry" | grep -q "true"
}
