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

function setup_registry_data_device() {
    local data_device="$1"
    local mount_point="/mnt/system-registry-data"
    local fstab_file="/etc/fstab"
    local symlink_target="/opt/deckhouse/system-registry"
    local label="registry-data"

    if ! [ -b "$data_device" ]; then
      >&2 echo "Failed to find $data_device disk."
      exit 1
    fi

    # Ensure the mount directory exists
    mkdir -p "$mount_point"

    # Format the data device if it is not already ext4
    if ! file -s "$data_device" | grep -q ext4; then
        mkfs.ext4 -F -L "$label" "$data_device"
    fi

    # Add an entry to /etc/fstab if it does not already exist
    if ! grep -q "$label" "$fstab_file"; then
        echo "LABEL=$label $mount_point ext4 defaults,discard,x-systemd.automount 0 0" >> "$fstab_file"
    fi

    # Mount the device if it is not already mounted
    if ! mount | grep -q "$mount_point"; then
        mount -L "$label"
    fi

    # Create a symlink if the target directory is empty
    if [[ "$(find "$symlink_target" -type f 2>/dev/null | wc -l)" == "0" ]]; then
        rm -rf "$symlink_target"
        ln -s "$mount_point" "$symlink_target"
    fi
}

function teardown_registry_data_device() {
    local mount_point="/mnt/system-registry-data"
    local fstab_file="/etc/fstab"
    local link_target="/opt/deckhouse/system-registry"
    local label="registry-data"
  
    # Remove the symbolic link if it exists and points to the correct location
    if [[ -L "$link_target" && "$(readlink "$link_target")" == "$mount_point" ]]; then
        rm -f "$link_target"
    fi
    
    # Remove the entry from /etc/fstab
    if grep -q "$label" "$fstab_file"; then
        sed -i "/^LABEL=${label}.*/d" "$fstab_file"
    fi
    
    # Unmount the device
    if mount | grep -q "$mount_point"; then
        umount "$mount_point"
    fi
}

{{- /*
# This function removes the system_registry_data_device_path file, which is used by the 
# function get_registry_data_device_from_file_or_from_secret
#
# Purpose:
#   After this step, the file must be deleted to ensure the system uses the latest data from the 
#   Kubernetes secret d8-masters-system-registry-data-device-path. This secret is updated after 
#   the "converge" operation and contains the actual device information.
*/}}
function remove_registry_data_device_file() {
  local data_device_file="$BOOTSTRAP_DIR/system_registry_data_device_path"
  if [ -f "$data_device_file" ]; then
    rm -f "$data_device_file"
  fi
}

function find_first_unmounted_data_device() {
  lsblk -o path,type,mountpoint,fstype --tree --json | jq -r \
    '[ .blockdevices[] | select (.path | contains("zram") | not ) | select ( .type == "disk" and .mountpoint == null and .children == null) | .path ] | sort | first'
}

function find_mounted_registry_data_device() {
  lsblk -o path,type,mountpoint,fstype --tree --json | jq -r \
    '[.blockdevices[] | select(.mountpoint == "/mnt/system-registry-data" ) | .path] | first'
}

{{- /*
# Example (lsblk -o path,type,mountpoint,fstype --tree --json):
#         {
#          "path": "/dev/vda",
#          "type": "disk",
#          "mountpoint": null,
#          "fstype": null,
#          "children": [
#             {
#                "path": "/dev/vda1",
#                "type": "part",
#                "mountpoint": "/",
#                "fstype": "ext4"
#             },{
#                "path": "/dev/vda15",
#                "type": "part",
#                "mountpoint": "/boot/efi",
#                "fstype": "vfat"
#             }
#          ]
#       },{
#          "path": "/dev/vdb",
#          "type": "disk",
#          "mountpoint": null,
#          "fstype": null
#       }
*/}}

function is_unmounted_data_device_exists() {
  local data_device
  data_device=$(find_first_unmounted_data_device)
  if [ "$data_device" != "null" ] && [ -n "$data_device" ]; then
    return 0
  else
    return 1
  fi
}

function is_registry_data_device_mounted() {
  local data_device
  data_device=$(find_mounted_registry_data_device)
  if [ "$data_device" != "null" ] && [ -n "$data_device" ]; then
    return 0
  else
    return 1
  fi
}

function create_registry_data_device_installed_file() {
  local installed_file="$BOOTSTRAP_DIR/system-registry-data-device-installed"
  touch "$installed_file"
}

function remove_registry_data_device_installed_file() {
  local installed_file="$BOOTSTRAP_DIR/system-registry-data-device-installed"
  if [ -f "$installed_file" ]; then
    rm -f "$installed_file"
  fi
}

# Check if the registry data device is already mounted
if is_registry_data_device_mounted; then
  # If mounted, create the installed file marker
  create_registry_data_device_installed_file
else
  # Read the device path from the system registry file
  dataDevice=$(cat "$BOOTSTRAP_DIR/system_registry_data_device_path")
  
  # If dataDevice is non-empty
  if [ -n "$dataDevice" ]; then
    # Check if the data device actually exists as a block device
    if ! [ -b "$dataDevice" ]; then
      # If it doesn't exist, log an error and attempt to detect the correct device
      >&2 echo "Failed to find $dataDevice disk. Detecting the correct one..."

      {{- /*
        # Sometimes the device path (`device_path`) returned by Terraform points to a non-existent device.
        # In such a situation, we want to find an unpartitioned unused device
        # without a file system, assuming that it is the correct one.
        # To form the mounting order of devices in Terraform, we specify mounting with the `depends` condition.
        # Additionally, we define the array of disks in Terraform when creating the instance machine.
      */}}
      dataDevice=$(find_first_unmounted_data_device)
    fi
    # Set up the registry data device
    setup_registry_data_device "$dataDevice"
    # Create the installed file marker after setup
    create_registry_data_device_installed_file
  else
    # If dataDevice is empty, teardown the registry data device
    teardown_registry_data_device
    # Remove the installed file marker
    remove_registry_data_device_installed_file
  fi
fi

# Clean up by removing the registry data device file
remove_registry_data_device_file

  {{- end  }}
{{- end  }}
