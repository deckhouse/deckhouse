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

# Install Azure udev rules for NVMe disk support (Gen2 VMs)
# This fixes disk discovery for Ubuntu 22.04 and other NVMe-based instances
# See: https://github.com/kubernetes-sigs/azuredisk-csi-driver/issues/2777

if [[ "$FIRST_BASHIBLE_RUN" != "yes" ]]; then
  exit 0
fi

# Check if running on Azure with NVMe controller
if ! lsblk -o NAME,MODEL | grep -q "MSFT NVMe"; then
  # Not an NVMe VM, skip
  exit 0
fi

UDEV_RULES_FILE="/etc/udev/rules.d/80-azure-disk.rules"

# Download official Azure udev rules for NVMe disk mapping
# These rules create /dev/disk/azure/data/by-lun/* symlinks for CSI driver
bb-log-info "Installing Azure NVMe udev rules for disk discovery"

cat > "$UDEV_RULES_FILE" << 'EOF'
ACTION!="add|change", GOTO="azure_disk_end"
SUBSYSTEM!="block", GOTO="azure_disk_end"

KERNEL=="nvme*", ATTRS{nsid}=="?*", ENV{ID_MODEL}=="Microsoft NVMe Direct Disk", GOTO="azure_disk_nvme_direct_v1"
KERNEL=="nvme*", ATTRS{nsid}=="?*", ENV{ID_MODEL}=="Microsoft NVMe Direct Disk v2", GOTO="azure_disk_nvme_direct_v2"
KERNEL=="nvme*", ATTRS{nsid}=="?*", ENV{ID_MODEL}=="MSFT NVMe Accelerator v1.0", GOTO="azure_disk_nvme_remote_v1"
ENV{ID_VENDOR}=="Msft", ENV{ID_MODEL}=="Virtual_Disk", GOTO="azure_disk_scsi"
GOTO="azure_disk_end"

LABEL="azure_disk_scsi"
ATTRS{device_id}=="?00000000-0000-*", ENV{AZURE_DISK_TYPE}="os", GOTO="azure_disk_symlink"
ENV{DEVTYPE}=="partition", PROGRAM="/bin/sh -c 'readlink /sys/class/block/%k/../device|cut -d: -f4'", ENV{AZURE_DISK_LUN}="$result"
ENV{DEVTYPE}=="disk", PROGRAM="/bin/sh -c 'readlink /sys/class/block/%k/device|cut -d: -f4'", ENV{AZURE_DISK_LUN}="$result"
ATTRS{device_id}=="{f8b3781a-1e82-4818-a1c3-63d806ec15bb}", ENV{AZURE_DISK_LUN}=="0", ENV{AZURE_DISK_TYPE}="os", ENV{AZURE_DISK_LUN}="", GOTO="azure_disk_symlink"
ATTRS{device_id}=="{f8b3781b-1e82-4818-a1c3-63d806ec15bb}", ENV{AZURE_DISK_TYPE}="data", GOTO="azure_disk_symlink"
ATTRS{device_id}=="{f8b3781c-1e82-4818-a1c3-63d806ec15bb}", ENV{AZURE_DISK_TYPE}="data", GOTO="azure_disk_symlink"
ATTRS{device_id}=="{f8b3781d-1e82-4818-a1c3-63d806ec15bb}", ENV{AZURE_DISK_TYPE}="data", GOTO="azure_disk_symlink"

# Use "resource" type for local SCSI because some VM skus offer NVMe local disks in addition to a SCSI resource disk, e.g. LSv3 family.
# This logic is already in walinuxagent rules but we duplicate it here to avoid an unnecessary dependency for anyone requiring it.
ATTRS{device_id}=="?00000000-0001-*", ENV{AZURE_DISK_TYPE}="resource", ENV{AZURE_DISK_LUN}="", GOTO="azure_disk_symlink"
ATTRS{device_id}=="{f8b3781a-1e82-4818-a1c3-63d806ec15bb}", ENV{AZURE_DISK_LUN}=="1", ENV{AZURE_DISK_TYPE}="resource", ENV{AZURE_DISK_LUN}="", GOTO="azure_disk_symlink"
GOTO="azure_disk_end"

LABEL="azure_disk_nvme_direct_v1"
LABEL="azure_disk_nvme_direct_v2"
ATTRS{nsid}=="?*", ENV{AZURE_DISK_TYPE}="local", ENV{AZURE_DISK_SERIAL}="$env{ID_SERIAL_SHORT}"
GOTO="azure_disk_nvme_id"

LABEL="azure_disk_nvme_remote_v1"
# Azure hosts will retry remote I/O requests for up to 120 seconds.  If I/O times out for longer than that, host will reboot OS.
# Set timeout for remote disks to 240 seconds, giving host time to handle the retry or reboot the VM.
ENV{DEVTYPE}=="disk", ATTRS{nsid}=="?*", ATTR{queue/io_timeout}="240000"

# For remote disks, namespace ID=1 is OS disk, ID=2+ are data disks with customer-configured lun=ID-2 (e.g. lun=0 will have nsid=2).
ATTRS{nsid}=="1", ENV{AZURE_DISK_TYPE}="os", GOTO="azure_disk_nvme_id"
ATTRS{nsid}=="?*", ENV{AZURE_DISK_TYPE}="data", PROGRAM="/bin/sh -ec 'echo $$((%s{nsid}-2))'", ENV{AZURE_DISK_LUN}="$result"

LABEL="azure_disk_nvme_id"
# Skip azure-nvme-id if not available (it's optional for basic functionality)
TEST=="/usr/sbin/azure-nvme-id", IMPORT{program}="/usr/sbin/azure-nvme-id --udev"

LABEL="azure_disk_symlink"
# systemd v254 ships an updated 60-persistent-storage.rules that would allow
# these to be deduplicated using $env{.PART_SUFFIX}
ENV{DEVTYPE}=="disk", ENV{AZURE_DISK_TYPE}=="os|resource|root", SYMLINK+="disk/azure/$env{AZURE_DISK_TYPE}"
ENV{DEVTYPE}=="disk", ENV{AZURE_DISK_TYPE}=="?*", ENV{AZURE_DISK_INDEX}=="?*", SYMLINK+="disk/azure/$env{AZURE_DISK_TYPE}/by-index/$env{AZURE_DISK_INDEX}"
ENV{DEVTYPE}=="disk", ENV{AZURE_DISK_TYPE}=="?*", ENV{AZURE_DISK_LUN}=="?*", SYMLINK+="disk/azure/$env{AZURE_DISK_TYPE}/by-lun/$env{AZURE_DISK_LUN}"
ENV{DEVTYPE}=="disk", ENV{AZURE_DISK_TYPE}=="?*", ENV{AZURE_DISK_NAME}=="?*", SYMLINK+="disk/azure/$env{AZURE_DISK_TYPE}/by-name/$env{AZURE_DISK_NAME}"
ENV{DEVTYPE}=="disk", ENV{AZURE_DISK_TYPE}=="?*", ENV{AZURE_DISK_SERIAL}=="?*", SYMLINK+="disk/azure/$env{AZURE_DISK_TYPE}/by-serial/$env{AZURE_DISK_SERIAL}"
ENV{DEVTYPE}=="partition", ENV{AZURE_DISK_TYPE}=="os|resource|root", SYMLINK+="disk/azure/$env{AZURE_DISK_TYPE}-part%n"
ENV{DEVTYPE}=="partition", ENV{AZURE_DISK_TYPE}=="?*", ENV{AZURE_DISK_INDEX}=="?*", SYMLINK+="disk/azure/$env{AZURE_DISK_TYPE}/by-index/$env{AZURE_DISK_INDEX}-part%n"
ENV{DEVTYPE}=="partition", ENV{AZURE_DISK_TYPE}=="?*", ENV{AZURE_DISK_LUN}=="?*", SYMLINK+="disk/azure/$env{AZURE_DISK_TYPE}/by-lun/$env{AZURE_DISK_LUN}-part%n"
ENV{DEVTYPE}=="partition", ENV{AZURE_DISK_TYPE}=="?*", ENV{AZURE_DISK_NAME}=="?*", SYMLINK+="disk/azure/$env{AZURE_DISK_TYPE}/by-name/$env{AZURE_DISK_NAME}-part%n"
ENV{DEVTYPE}=="partition", ENV{AZURE_DISK_TYPE}=="?*", ENV{AZURE_DISK_SERIAL}=="?*", SYMLINK+="disk/azure/$env{AZURE_DISK_TYPE}/by-serial/$env{AZURE_DISK_SERIAL}-part%n"
LABEL="azure_disk_end"
EOF

# Reload udev rules and trigger re-evaluation
udevadm control --reload-rules
udevadm trigger

bb-log-info "Azure NVMe udev rules installed and activated"

# Verify symlinks were created
if [ -d /dev/disk/azure/data ]; then
  bb-log-info "Azure disk symlinks created successfully:"
  ls -la /dev/disk/azure/data/by-lun/ 2>/dev/null || bb-log-warning "No data disks found yet"
else
  bb-log-warning "Azure disk symlinks directory not created - disks may appear after reboot or disk attachment"
fi
