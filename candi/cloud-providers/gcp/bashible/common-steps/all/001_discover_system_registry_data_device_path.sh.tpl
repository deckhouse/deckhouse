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
  local device_name="$(lsblk -lo name,serial | grep "$cloud_disk_name" | cut -d " " -f1)"
  if [ -z "$device_name" ]; then
    >&2 echo "failed to discover system-registry-data device"
    exit 1
  fi
  echo "/dev/$device_name"
}

# The system registry file is always created in step 000_create_system_registry_data_device_path.sh.tpl
system_registry_file="/var/lib/bashible/system_registry_data_device_path"

# Get system registry data device
CLOUD_DISK_NAME_OR_DATA_DEVICE=$(cat "$system_registry_file")

if [ -n "$CLOUD_DISK_NAME_OR_DATA_DEVICE" ] && [[ "$CLOUD_DISK_NAME_OR_DATA_DEVICE" != /dev/* ]]; then
  DATA_DEVICE=$(discover_device_path "$CLOUD_DISK_NAME_OR_DATA_DEVICE")
  echo "system-registry-data device: $DATA_DEVICE"
  echo "$DATA_DEVICE" > "$system_registry_file"
fi

  {{- end  }}
{{- end  }}
