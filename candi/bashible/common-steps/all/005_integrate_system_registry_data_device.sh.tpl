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

    if is_data_device_mounted "$data_device"; then
      >&2 echo "Failed to mount $data_device disk. Disk is already mounted."
      exit 1
    fi

    # Ensure the mount directory exists
    mkdir -p "$mount_point"

    # Format the data device if it is not already ext4
    if ! file -s "$data_device" | grep -q ext4; then
      mkfs.ext4 -F -L "$label" "$data_device"
    else
      # else add the label
      /opt/deckhouse/bin/tune2fs -L "$label" "$data_device"
    fi

    # Add an entry to /etc/fstab if it does not already exist
    if ! grep -q "$label" "$fstab_file"; then
      echo "LABEL=$label $mount_point ext4 defaults,discard,x-systemd.automount 0 0" >> "$fstab_file"
    fi

    # Mount the device if it is not already mounted
    if ! mount | grep -q "$mount_point"; then
      mount -L "$label"
    fi

    # Check if symlink_target exists
    if [[ -e "$symlink_target" ]]
    then
      # Check if symlink_target is a symlink and points to mount_point
      if [[ -L "$symlink_target" && $(readlink -f "$symlink_target") == "$mount_point" ]]; then
        echo "symlink is correct, nothing to do"
      else
        rm -rf "$symlink_target"
        ln -s "$mount_point" "$symlink_target"
      fi
    else
      ln -s "$mount_point" "$symlink_target"
    fi
}

function teardown_registry_data_device() {
    local mount_point="/mnt/system-registry-data"
    local fstab_file="/etc/fstab"
    local link_target="/opt/deckhouse/system-registry"
    local label="registry-data"
  
    # Remove the symbolic link if it exists
    if [[ -L "$link_target" ]]; then
        rm -f "$link_target"
    fi
    
    # Remove the entry from /etc/fstab
    if grep -q "$label" "$fstab_file"; then
        sed -i "/^LABEL=${label}.*/d" "$fstab_file"
    fi

    # Remove the mount point if it exists
    if [[ -e "$mount_point" ]]; then
        rm -rf "$mount_point"
    fi
}

function check_if_symlink_and_return_target() {
    local input_data="$1"
    
    if [[ -L "$input_data" ]]; then
        echo "$(readlink -f "$input_data")"
    else 
        echo "$input_data"
    fi
}

function find_all_unmounted_data_devices() {
  lsblk -o path,type,mountpoint,fstype --tree --json | jq -r '
    [
      .blockdevices[] 
      | select(.path | contains("zram") | not)  # Exclude zram devices
      | select(.path | contains("fd") | not)   # Exclude floppy devices (fd)
      | select(.type == "disk" and .mountpoint == null and .children == null)  # Filter disks with no mountpoint or children
      | .path
    ] | sort'
}

function find_path_by_data_device_mountpoint() {
  local data_device_mountpoint="$1"
  lsblk -o path,type,mountpoint,fstype --tree --json | jq -r "
    [
      .blockdevices[] 
      | select(.mountpoint == \"$data_device_mountpoint\")  # Match the specific device mountpoint
      | .path
    ] | first"
}

function find_mountpoint_by_data_device() {
  local data_device="$1"
  lsblk -o path,type,mountpoint,fstype --tree --json | jq -r "
    [
      .blockdevices[] 
      | select(.path == \"$data_device\")  # Match the specific device path
      | .mountpoint
    ] | first"
}

function find_path_by_data_device_label() {
  local device_label="$1"
  
  local device_path=$(lsblk -o path,type,mountpoint,fstype,label --tree --json | jq -r "
    [
      .blockdevices[] 
      | select(.label == \"$device_label\")  # Match the specific device label
    ] | first | .path
  ")
  if [[ "$device_path" != "null" ]]; then
    echo "$device_path"
  fi
}

function find_first_unmounted_data_device() {
  local all_unmounted_data_devices="$(find_all_unmounted_data_devices)"
  echo "$all_unmounted_data_devices" | jq '. | first'
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

function is_data_device_mounted() {
  local data_device="$1"
  local data_device_info

  data_device_info=$(find_mountpoint_by_data_device "$data_device")

  if [ "$data_device_info" != "null" ] && [ -n "$data_device_info" ]; then
    return 0
  else
    return 1
  fi
}

function is_registry_data_device_mounted() {
  local registry_data_device_mountpoint="/mnt/system-registry-data"
  local data_device
  data_device=$(find_path_by_data_device_mountpoint "$registry_data_device_mountpoint")
  if [ "$data_device" != "null" ] && [ -n "$data_device" ]; then
    return 0
  else
    return 1
  fi
}

function create_registry_data_device_installed_file() {
  local installed_file="/var/lib/bashible/system-registry-data-device-installed"
  touch "$installed_file"
}

function remove_registry_data_device_installed_file() {
  local installed_file="/var/lib/bashible/system-registry-data-device-installed"
  if [ -f "$installed_file" ]; then
    rm -f "$installed_file"
  fi
}

function enable_registry_data_device_label() {
  if [[ "$FIRST_BASHIBLE_RUN" == "yes" ]]; then
    return 0
  fi

  local label="node.deckhouse.io/registry-data-device-ready=true"
  local node="$D8_NODE_HOSTNAME"

  echo "Label node $node with labels $label"
  error=$(bb-kubectl --request-timeout=60s --kubeconfig=/etc/kubernetes/kubelet.conf label node $node --overwrite $label 2>&1)
  if [ $? -ne 0 ]; then
    >&2 echo "Failed to label node $node. Error from kubectl: ${error}"
    exit 1
  fi
  echo "Successful label node $node with labels $label"
}

function disable_registry_data_device_label() {
  if [[ "$FIRST_BASHIBLE_RUN" == "yes" ]]; then
    return 0
  fi

  local label="node.deckhouse.io/registry-data-device-ready="
  local node="$D8_NODE_HOSTNAME"

  echo "Label node $node with labels $label"
  error=$(bb-kubectl --request-timeout=60s --kubeconfig=/etc/kubernetes/kubelet.conf label node $node --overwrite $label 2>&1)
  if [ $? -ne 0 ]; then
    >&2 echo "Failed to label node $node. Error from kubectl: ${error}"
    exit 1
  fi
  echo "Successful label node $node with labels $label"
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


# Skip for
if check_annotation "embedded-registry.deckhouse.io/lock-data-device-mount"; then
    exit 0
fi

# If it does not override
if [ ! -f /var/lib/bashible/system_registry_data_device_path ]; then
  echo "$(bb-get-registry-data-device-from-terraform-output)" > "/var/lib/bashible/system_registry_data_device_path"
fi

# Check if the registry data device is already mounted
if is_registry_data_device_mounted; then
  # If mounted, create the installed file marker
  create_registry_data_device_installed_file
  enable_registry_data_device_label
else
  # Read the device path from the system registry file
  # The file always exists (created in step 000_create_system_registry_data_device_path.sh.tpl)
  dataDevice=$(cat "/var/lib/bashible/system_registry_data_device_path")
  
  # If dataDevice is non-empty
  if [ -n "$dataDevice" ]; then

    # for converge
    dataDeviceByLabel=$(find_path_by_data_device_label "registry-data")
    if [ -n "$dataDeviceByLabel" ]; then
      dataDevice="$dataDeviceByLabel"
    fi

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
    dataDevice=$(check_if_symlink_and_return_target "$dataDevice")
    # Set up the registry data device
    setup_registry_data_device "$dataDevice"
    # Create the installed file marker after setup
    create_registry_data_device_installed_file
    enable_registry_data_device_label
  else
    # Disable label before teardown
    disable_registry_data_device_label
    sleep 5
    # If dataDevice is empty, teardown the registry data device
    teardown_registry_data_device
    # Remove the installed file marker
    remove_registry_data_device_installed_file
  fi
fi

rm -f /var/lib/bashible/system_registry_data_device_path

  {{- end  }}
{{- end  }}
