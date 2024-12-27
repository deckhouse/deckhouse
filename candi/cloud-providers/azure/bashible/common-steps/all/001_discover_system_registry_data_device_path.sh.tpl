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
  local lun_name="$1"

  # Full device path via /dev/disk/azure/*/$lun_name
  local device_path="$(ls -1 /dev/disk/azure/*/$lun_name)"
  if [ "$(wc -l <<< "$device_path")" -ne 1 ]; then
    >&2 echo "Failed to discover device by lun: $lun_name"
    exit 1
  fi

  # Check if the symbolic link exists
  if [ ! -e "$device_path" ]; then
    >&2 echo "Failed to discover device: $device_path not found"
    exit 1
  fi
  
  # Resolve the symbolic link to the real path
  device_path=$(readlink -f "$device_path")

  # Check that the path is resolved and exists
  if [ -z "$device_path" ] || [ ! -b "$device_path" ]; then
    >&2 echo "Failed to resolve device path for: $lun_name"
    exit 1
  fi
  
  # Return the real device path
  echo "$device_path"
}

function check_annotation(){
    local annotation="$1"
    local node="$D8_NODE_HOSTNAME"
    local node_annotations=$(bb-kubectl --request-timeout=60s --kubeconfig=/etc/kubernetes/kubelet.conf get node $node -o json | jq '.metadata.annotations')

    if echo "$node_annotations" | jq 'has("'$annotation'")' | grep -q 'true'; then
        return 0
    fi
    return 1
}


# Set empty string to escape mount by 005_integrate_system_registry_data_device.sh
if check_annotation "embedded-registry.deckhouse.io/lock-data-device-mount"; then
  echo "" > "/var/lib/bashible/system_registry_data_device_path"
  exit 0
fi

# Get system registry data device
DATA_DEVICE="$(bb-get-registry-data-device-from-terraform-output)"

if [ -n "$DATA_DEVICE" ] && [[ "$DATA_DEVICE" != /dev/* ]]; then
  DATA_DEVICE=$(discover_device_path "lun11")
  echo "system-registry-data device: $DATA_DEVICE"
  echo "$DATA_DEVICE" > "/var/lib/bashible/system_registry_data_device_path"
fi

  {{- end  }}
{{- end  }}
