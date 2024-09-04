{{- if ne .nodeGroup.nodeType "Static" }}
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
export LC_MESSAGES=en_US.UTF 

grow_partition() {
  # /dev/sda1 => /sys/block/*/sda1/partition
  sysfs_path="/sys/block/*/${1#"/dev/"}/partition"

  # get partition number
  if ls ${sysfs_path} >/dev/null 2>&1; then
    partition_number="$(cat ${sysfs_path})"
  else
    # disk without partition, do nothing
    return 0
  fi

  disk="/dev/$(ls ${sysfs_path} 2>/dev/null | awk -F "/" '{print $4; exit;}')"

  # if partition number >=5 this is or logical partition and we must resize extended partition first, or non-mbr partition
  if [[ "${partition_number}" -ge 5 ]]; then
    # find extended partition number
    ext_partition="$(fdisk -l "${disk}" | grep -i extended | awk '{print $1}')"
		ext_partition_number=${ext_partition#disk}

    # Mbr extended partition
    if [[ -n "${ext_partition_number}" ]]; then
      if ! growpart --dry-run "${disk}" "${ext_partition_number}" >/dev/null; then
        return 1
      fi

      growpart "${disk}" "${ext_partition_number}"
    fi
  fi

  if ! growpart --dry-run "${disk}" "${partition_number}" >/dev/null; then
    return 1
  fi

  growpart "${disk}" "${partition_number}"
}

grow_lvm() {
  # suppress message from lvm `File descriptor leaked on lvs invocation`
  # LVM_SUPPRESS_FD_WARNINGS=1

  vgname="$(LVM_SUPPRESS_FD_WARNINGS=1 lvdisplay -c "${1}" | cut -d ":" -f2)"
  for pv in $(LVM_SUPPRESS_FD_WARNINGS=1 pvdisplay -c -S vgname="${vgname}" | cut -d ":" -f1 | tr -d " "); do
    if ! grow_partition "${pv}" ; then
      continue
    fi
    LVM_SUPPRESS_FD_WARNINGS=1 pvresize "${pv}"
  done
  LVM_SUPPRESS_FD_WARNINGS=1 lvextend -q -l +100%FREE "${1}"
}

# resize only ext4 disks, because we don't use other fs as node disks.
# exclude from resize all csi-driven disks, and disks, managed by kubelet (like https://kubernetes.io/docs/concepts/storage/volumes/#rbd).
for partition in $(mount | grep -vE "kubernetes.io" | grep "ext4" | awk '{print $1}' | sort -u); do
  # check if disk is present
  if [[ ! -e "${partition}" ]]; then
    continue
  fi

  # partition = /dev/mapper/vgubuntu-root. LVM partition.
  if [[ "${partition}" =~ ^/dev/mapper/[a-z\-]+$ ]]; then
    if grow_lvm "${partition}"; then
      resize2fs "${partition}"
    fi
  # all other partitions
  else
    if grow_partition "${partition}" ; then
      resize2fs "${partition}"
    fi
  fi

done
{{- end }}
