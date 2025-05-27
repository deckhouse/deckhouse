# Copyright 2024 Flant JSC
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

{{- $nodeTypeList := list "CloudPermanent" }}
{{- if has .nodeGroup.nodeType $nodeTypeList }}
  {{- if eq .nodeGroup.name "master" }}

function discover_device_path() {
  local cloud_disk_name="$1"

  # Full device path via /dev/disk/by-id/
  local device_path="/dev/disk/by-id/google-${cloud_disk_name}"

  # Check if the symbolic link exists
  if [ ! -e "$device_path" ]; then
    >&2 echo "Failed to discover device: $device_path not found"
    exit 1
  fi
  
  # Resolve the symbolic link to the real path
  device_path=$(readlink -f "$device_path")

  # Check that the path is resolved and exists
  if [ -z "$device_path" ] || [ ! -b "$device_path" ]; then
    >&2 echo "Failed to resolve device path for: $cloud_disk_name"
    exit 1
  fi
  
  # Return the real device path
  echo "$device_path"
}

function is_annotation_exist(){
    local annotation="$1"
    local node="$D8_NODE_HOSTNAME"
    local node_annotations=$(bb-kubectl --request-timeout=60s --kubeconfig=/etc/kubernetes/kubelet.conf get node $node -o json | jq '.metadata.annotations')

    if echo "$node_annotations" | jq 'has("'$annotation'")' | grep -q 'true'; then
        return 0
    fi
    return 1
}


# Set empty string to escape mount by 005_integrate_system_registry_data_device.sh
if is_annotation_exist "embedded-registry.deckhouse.io/data-device-mount-lock"; then
  echo "" > "/var/lib/bashible/system_registry_data_device_path"
  exit 0
fi

# Get system registry data device
CLOUD_DISK_NAME_OR_DATA_DEVICE="$(bb-get-registry-data-device-from-terraform-output)"

if [ -n "$CLOUD_DISK_NAME_OR_DATA_DEVICE" ] && [[ "$CLOUD_DISK_NAME_OR_DATA_DEVICE" != /dev/* ]]; then
  DATA_DEVICE=$(discover_device_path "$CLOUD_DISK_NAME_OR_DATA_DEVICE")
  echo "system-registry-data device: $DATA_DEVICE"
  echo "$DATA_DEVICE" > "/var/lib/bashible/system_registry_data_device_path"
fi

  {{- end  }}
{{- end  }}
