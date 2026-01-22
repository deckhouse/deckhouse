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
  local first_unmounted_data_device=$(echo "$all_unmounted_data_devices" | jq '. | first')
  if [ "$first_unmounted_data_device" != "null" ] && [ -n "$first_unmounted_data_device" ]; then
    echo "$first_unmounted_data_device"
  else
    echo ""
  fi
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

function check_expected_disk_count() {
  local expected_disks_count=1  # For Kubernetes data

  # If the registry data device exists in terraform output
  if [ -n "$(bb-get-registry-data-device-from-terraform-output)" ]; then
    expected_disks_count=$((expected_disks_count + 1))
  fi

  # Find all unmounted data devices and count them
  local all_unmounted_data_devices=$(find_all_unmounted_data_devices)
  local all_unmounted_data_devices_count=$(echo "$all_unmounted_data_devices" | jq '. | length')

  # Compare the count of found devices with the expected count
  if [ "$all_unmounted_data_devices_count" -ne "$expected_disks_count" ]; then
    >&2 echo "Received disks: $all_unmounted_data_devices, expected count: $expected_disks_count"
    exit 1
  fi
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
DATA_DEVICE=$(bb-get-kubernetes-data-device-from-file-or-secret)
if [ -z "$DATA_DEVICE" ]; then
  >&2 echo "failed to get kubernetes data device path"
  exit 1
fi

# For converge
DATA_DEVICE_BY_LABEL=$(find_path_by_data_device_label "kubernetes-data")
if [ -n "$DATA_DEVICE_BY_LABEL" ]; then
  DATA_DEVICE="$DATA_DEVICE_BY_LABEL"
fi

if ! [ -b "$DATA_DEVICE" ]; then
  >&2 echo "Failed to find $DATA_DEVICE disk. Trying to detect the correct one..."

  {{- /*
    # Sometimes the device path (`device_path`) returned by Terraform points to a non-existent device.
    # In such a situation, we want to find an unpartitioned unused device
    # without a file system, assuming that it is the correct one.
    # To form the mounting order of devices in Terraform, we specify mounting with the `depends` condition.
    # Additionally, we define the array of disks in Terraform when creating the instance machine.
  */}}

  # Wait all disks and then get first unmounted data device
  check_expected_disk_count
  DATA_DEVICE=$(find_first_unmounted_data_device)
  if ! [ -b "$DATA_DEVICE" ]; then
    >&2 echo "Failed to find a valid disk by lsblk."
    exit 1
  fi
fi

# Mount kubernetes data device steps:
mkdir -p /mnt/kubernetes-data

# always format the device to ensure it's clean, because etcd will not join the cluster if the device
# contains a filesystem with etcd database from a previous installation
mkfs.ext4 -F -L kubernetes-data $DATA_DEVICE

if grep -qv kubernetes-data /etc/fstab; then
  cat >> /etc/fstab << EOF
LABEL=kubernetes-data           /mnt/kubernetes-data     ext4   defaults,discard,x-systemd.automount        0 0
EOF
fi

if ! mount | grep -q $DATA_DEVICE; then
  mount -L kubernetes-data
fi

mkdir -p /mnt/kubernetes-data/var-lib-etcd
chmod 700 /mnt/kubernetes-data/var-lib-etcd
mkdir -p /mnt/kubernetes-data/etc-kubernetes

# if there is kubernetes dir with regular files then we can't delete it
# if there aren't files then we can delete dir to prevent symlink creation problems
if [[ "$(find /etc/kubernetes/ -type f 2>/dev/null | wc -l)" == "0" ]]; then
  rm -rf /etc/kubernetes
  ln -s /mnt/kubernetes-data/etc-kubernetes /etc/kubernetes
fi

if [[ "$(find /var/lib/etcd/ -type f 2>/dev/null | wc -l)" == "0" ]]; then
  rm -rf /var/lib/etcd
  ln -s /mnt/kubernetes-data/var-lib-etcd /var/lib/etcd
fi

touch /var/lib/bashible/kubernetes-data-device-installed

  {{- end  }}
{{- end  }}
