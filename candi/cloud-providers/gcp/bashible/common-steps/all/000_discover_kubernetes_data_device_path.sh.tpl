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

# Skip for
if [[ "$FIRST_BASHIBLE_RUN" != "yes" ]]; then
  exit 0
fi

# Skip for
if [ -f /var/lib/bashible/kubernetes-data-device-installed ]; then
  exit 0
fi

# Get Kubernetes data device
CLOUD_DISK_NAME_OR_DATA_DEVICE=$(bb-get-kubernetes-data-device-from-file-or-secret)
if [ -z "$CLOUD_DISK_NAME_OR_DATA_DEVICE" ]; then
  >&2 echo "failed to get kubernetes data device path"
  exit 1
fi

if [[ "$CLOUD_DISK_NAME_OR_DATA_DEVICE" != /dev/* ]]; then
  DATA_DEVICE=$(discover_device_path "$CLOUD_DISK_NAME_OR_DATA_DEVICE")
  echo "kubernetes-data device: $DATA_DEVICE"
  echo "$DATA_DEVICE" > /var/lib/bashible/kubernetes_data_device_path
fi

  {{- end  }}
{{- end  }}
