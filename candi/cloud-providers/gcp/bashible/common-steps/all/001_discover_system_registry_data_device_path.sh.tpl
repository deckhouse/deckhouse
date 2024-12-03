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

# Function to discover the device path based on the cloud disk name
function discover_device_path() {
  local cloud_disk_name="$1"
  # Use lsblk to find the device name associated with the given cloud disk serial
  device_name="$(lsblk -lo name,serial | grep "$cloud_disk_name" | cut -d " " -f1)"

  # Check if a device name was discovered
  if [ -z "$device_name" ]; then
    >&2 echo "Failed to discover system-registry-data device for cloud disk name: $cloud_disk_name"
    exit 1
  fi

  # Return the full device path
  echo "/dev/$device_name"
}

# The system registry file is always created in step 000_create_system_registry_data_device_path.sh.tpl
# and it is removed after the process completes in step 005_integrate_system_registry_data_device.sh.tpl
system_registry_file="$BOOTSTRAP_DIR/system_registry_data_device_path"

# Read the cloud disk name from the system registry file
cloud_disk_name=$(cat "$system_registry_file")

# Proceed only if the cloud_disk_name is non-empty and doesn't already start with /dev
if [ -n "$cloud_disk_name" ] && [[ "$cloud_disk_name" != /dev/* ]]; then
  # Discover the device path using the cloud disk name
  dataDevice=$(discover_device_path "$cloud_disk_name")
  echo "system_registry_data_device: $dataDevice"
  echo "$dataDevice" > /var/lib/bashible/system_registry_data_device_path
fi

# List block devices for diagnostic purposes
blkid

  {{- end  }}
{{- end  }}
