---
title: "Attaching disks and images to a virtual machine"
permalink: en/user/virtualization/vm-block-devices.html
description: "Attaching disks and images to a virtual machine through the specification and the VirtualMachineBlockDeviceAttachment resource, boot order, and device naming."
search: attaching a disk, VirtualMachineBlockDeviceAttachment, VMBDA, CD-ROM, boot order
---

You can attach disks and images to a virtual machine. They're described as block devices (BlockDevices).

Block device types and access modes:

| Block device type                                                          | Comment                                                           |
|----------------------------------------------------------------------------|-------------------------------------------------------------------|
| [VirtualImage](/modules/virtualization/cr.html#virtualimage)               | Attached in read-only mode, or as a CD-ROM for ISO images.        |
| [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) | Attached in read-only mode, or as a CD-ROM for ISO images.        |
| [VirtualDisk](/modules/virtualization/cr.html#virtualdisk)                 | Attached in read-write mode.                                      |

There are two ways to attach devices:

- Through the VM specification ([`.spec.blockDeviceRefs`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-blockdevicerefs)): The disks are listed in the [VirtualMachine](/modules/virtualization/cr.html#virtualmachine) configuration, and the boot order is set for them (by position in the list or through the `bootOrder` field). Recommended when configuring a VM manually, and when you need control over the boot order (for example, an ISO for OS installation).
- Through [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) (`vmbda`): The disk is attached as a separate resource and doesn't take part in the boot order. Disks are attached through the `virtio-scsi` bus, regardless of the `enableParavirtualization` value. Recommended for automation and when you don't have the rights to edit the VM.

With `enableParavirtualization: true`, both ways let you attach and detach disks on a running VM without a reboot, if the disk is available on the node where it runs. With `enableParavirtualization: false`, the contents of [`.spec.blockDeviceRefs`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-blockdevicerefs) on a running VM change only after a reboot; to attach and detach disks without a reboot, use [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) (`vmbda`).

{% alert level="warning" %}
When paravirtualization is disabled (`enableParavirtualization: false`), the devices from [`.spec.blockDeviceRefs`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-blockdevicerefs) work on the SATA bus, and on the IDE bus for the `Legacy` OS type. On a running VM, changes to this list, including attaching and detaching an ISO image, take effect only after a reboot.

Disks attached through [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) use the `virtio-scsi` bus and are attached without a reboot, if the guest OS has the driver for this bus. For the `Legacy` OS type with paravirtualization disabled, such an attachment is rejected, because the IDE bus doesn't support attaching on the fly: add the device to [`.spec.blockDeviceRefs`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-blockdevicerefs) and restart the VM.
{% endalert %}

You can attach a disk to a running VM only when the storage is available on the cluster node where the virtual machine runs. When creating and updating a VM, and when creating a [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment), the placement rules (`nodeSelector`, `affinity`, `tolerations`) of the volume, the virtual machine, and the VM class are taken into account, and they have to share at least one valid placement. If the VM is already running on a specific node, the new disk has to be available on that node.

While a live migration of the machine is preparing the target node, a disk can be neither attached nor detached. A new [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) resource stays in the `Pending` phase with the `BlockedByMigration` reason in the `Attached` condition, and a deleted one stays in the `Terminating` phase, and both finish once the migration completes. While the migration is still queued and the target node isn't being prepared yet, attaching and detaching work as usual.

## Attaching through the VM specification

The devices listed in the machine specification are attached at startup and stay in place for the whole run.

{% tabs bd-spec %}

{% tab "Using the CLI" %}

The list of block devices is set in the [`.spec.blockDeviceRefs`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-blockdevicerefs) field of the [VirtualMachine](/modules/virtualization/cr.html#virtualmachine) resource.

By default, the boot order matches the order of the devices in the list, and the optional `bootOrder` field lets you set it explicitly (a lower value means a higher priority). If `bootOrder` is specified for at least one device, only the devices with a set `bootOrder` get into the boot chain. Integers from 1 and up are allowed, unique within the list. When a device is removed from the list, the boot order is recalculated for the remaining devices.

A change to the order of devices in the list or to the `bootOrder` values takes effect after the VM reboots. For example, you can attach an ISO image for OS installation with the boot priority you need, and remove it from the list after the installation. If the VM has paravirtualization disabled (`enableParavirtualization: false`), edits to [`.spec.blockDeviceRefs`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-blockdevicerefs) on a running VM, including ones with an ISO image, apply after the VM reboots.

A fragment of the virtual machine configuration with block devices and an explicit boot order:

```yaml
spec:
  blockDeviceRefs:
    - kind: VirtualDisk
      name: <VD_NAME>
      bootOrder: 1
    - kind: VirtualImage
      name: <VI_NAME>
      bootOrder: 2
```

To attach a disk to a running virtual machine, add it to the [`.spec.blockDeviceRefs`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-blockdevicerefs) list:

```yaml
spec:
  blockDeviceRefs:
    - kind: VirtualDisk
      name: <VD_NAME>
    - kind: VirtualImage
      name: <VI_NAME>
    - kind: VirtualDisk
      name: <ADDITIONAL_DISK_NAME>
```

To detach a disk, remove it from the list. With `enableParavirtualization: false`, a change to the list on a running VM takes effect after the VM reboots.

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Virtual machines**.
1. Select the VM you need from the list and click its name.
1. On the **Configuration** tab, scroll down to the **Disks** section.
1. The following actions are available in the disk list:
   - **Add**: Attach a new disk or image to the VM.
   - **Eject**: Detach the device from the VM (the image or disk stays in the project, and you can attach it again to this or another VM).
   - **Delete**: Delete the image or disk resource itself from the cluster (after deletion, you can't reuse it).
   - Change the disk size, with the pencil icon next to the current size.
   - Change the boot order, by changing the position of the disk in the list.

{% endtab %}

{% endtabs %}

## Attaching through VirtualMachineBlockDeviceAttachment

A separate resource attaches a device to a machine without touching its specification.

{% tabs bd-vmbda %}

{% tab "Using the CLI" %}

The [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) resource attaches and detaches a block device on a VM without changing its specification. It suits automation and scenarios where the user doesn't have the rights to edit the VM.

Create a resource that attaches the empty `blank-disk` disk to the `linux-vm` virtual machine:

```shell
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineBlockDeviceAttachment
metadata:
  name: attach-blank-disk
spec:
  blockDeviceRef:
    kind: VirtualDisk
    name: blank-disk
  virtualMachineName: linux-vm
EOF
```

The device is attached when the resource moves to the `Attached` phase. The other phases are described in the [`.status.phase`](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment-v1alpha2-status-phase) field, and the [`.status.conditions`](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment-v1alpha2-status-conditions) block shows the reason for a delay.

Check the state of your resource:

```bash
d8 k get vmbda attach-blank-disk
```

Example output:

```console
NAME                PHASE      VIRTUALMACHINE   AGE
attach-blank-disk   Attached   linux-vm         3m7s
```
{: .nowrap-default }

Connect to the virtual machine and make sure the disk is attached:

```bash
d8 v ssh cloud@linux-vm --command "lsblk"
```

Example output:

```console
NAME    MAJ:MIN RM  SIZE RO TYPE MOUNTPOINTS
sda       8:0    0   10G  0 disk <--- statically attached linux-vm-root disk
|-sda1    8:1    0  9.9G  0 part /
|-sda14   8:14   0    4M  0 part
`-sda15   8:15   0  106M  0 part /boot/efi
sdb       8:16   0    1M  0 disk <--- cloudinit
sdc       8:32   0 95.9M  0 disk <--- dynamically attached blank-disk disk
```
{: .nowrap-default }

To detach the disk from the virtual machine, delete the resource you created earlier:

```bash
d8 k delete vmbda attach-blank-disk
```

Images are attached the same way, only the `kind` field takes the [VirtualImage](/modules/virtualization/cr.html#virtualimage) or [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) value.

```bash
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineBlockDeviceAttachment
metadata:
  name: attach-ubuntu-iso
spec:
  blockDeviceRef:
    kind: VirtualImage # or ClusterVirtualImage
    name: ubuntu-iso
  virtualMachineName: linux-vm
EOF
```

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Virtual machines**.
1. Select the VM you need from the list and click its name.
1. On the **Configuration** tab, scroll down to the **Disks** section.
1. The following actions are available in the disk list:
   - **Add**: Attach a new disk or image to the VM; to attach the device as an additional one (through [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment), without rebooting the VM), select the **Additional** checkbox in the **Disks / Images** window.
   - **Eject**: Detach the device from the VM (the image or disk stays in the project, and you can attach it again to this or another VM).
   - **Delete**: Delete the image or disk resource itself from the cluster (after deletion, you can't reuse it).
   - Change the disk size, with the pencil icon next to the current size.

{% endtab %}

{% endtabs %}

## Disk naming in the guest OS

Disk names in the guest system aren't stable, so relying on them in configuration is risky.

{% alert level="warning" %}
Block device names (`/dev/sda`, `/dev/sdb`, `/dev/sdc`, and so on) are assigned by the Linux kernel in the order the devices are discovered at boot. This order can change between reboots, so device names can change even when the SCSI addresses stay the same.

If you use `/dev/sdX` in configuration files (for example, `/etc/fstab`) or in scripts, after a VM reboot you can mount the wrong disk or end up with a malfunctioning system.
{% endalert %}

**Example:**

After the first VM boot:

```console
$ lsscsi
[0:0:0:1]  disk    QEMU     QEMU HARDDISK   /dev/sda
[0:0:0:2]  disk    QEMU     QEMU HARDDISK   /dev/sdb
```
{: .nowrap-default }

After a VM reboot:

```console
$ lsscsi
[0:0:0:1]  disk    QEMU     QEMU HARDDISK   /dev/sdb
[0:0:0:2]  disk    QEMU     QEMU HARDDISK   /dev/sda
```
{: .nowrap-default }

The SCSI addresses (`0:0:0:1`, `0:0:0:2`) stay the same, but the device names (`/dev/sda`, `/dev/sdb`) swap places.

Use stable identifiers instead of `/dev/sdX`:

- `/dev/disk/by-uuid/`: By partition UUID (preferable for `/etc/fstab`).
- `/dev/disk/by-path/`: By the SCSI connection path.
- `/dev/disk/by-id/`: By the SCSI device ID.

In configuration files and scripts, use partition UUIDs or symbolic links from `/dev/disk/by-*` instead of `/dev/sdX` names.

## Network interface naming in the guest OS

In systems without predictable network interface naming, network interface names (`eth0`, `eth1`, `eth2`, and so on) are assigned by the Linux kernel in the order the devices are discovered at boot. When you add new network interfaces or change the order of networks in [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks), the interface order can change, and IP addresses can end up assigned to the wrong interfaces.

Using `ethX` in configuration files (for example, `/etc/network/interfaces`, `netplan`, `systemd-networkd`) or in scripts when adding new interfaces or changing the order of networks can lead to network failures or a connection to the wrong network.

Modern distributions with systemd (Ubuntu 16.04+, Debian 9+, CentOS 7+, RHEL 7+) use predictable interface names by default (`enpXsY`, `ensX`, `enoX`), which are based on the physical characteristics of the device (PCI coordinates) and stay stable between reboots and when new interfaces are added.

But even with predictable names, bind the network configuration to the MAC addresses of the interfaces, especially if the order of networks in [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks) changes or new interfaces are added.

An example for systems without predictable naming:

Initially, the VM has two interfaces:

```console
$ ip link show
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536
2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500
3: eth1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500
```
{: .nowrap-default }

After adding a new interface at the beginning of the [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks) list and rebooting the VM:

```console
$ ip link show
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536
2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500  # New interface
3: eth1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500  # Former eth0
4: eth2: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500  # Former eth1
```
{: .nowrap-default }

The MAC addresses stay the same, but the interface names (`eth0`, `eth1`) shift, which can lead to IP addresses being assigned to the wrong interfaces.

Use stable identifiers instead of `ethX`:

- `enpXsY`: Predictable names based on the physical location (systemd networkd naming scheme, enabled by default in modern systems).
- Binding by MAC address: In the `netplan`, `systemd-networkd`, or `/etc/network/interfaces` configuration (preferable for guaranteed stability).

In configuration files and scripts, use stable interface names (`enpXsY`) or binding by MAC address instead of `ethX` names.

{% alert level="info" %}
The predictable interface order holds only in guest operating systems with systemd (for example, Ubuntu, Debian). In Alpine and other distributions without systemd, the order may differ.
{% endalert %}

To open a machine application to other machines or to users outside the cluster, configure a service or Ingress as described in [Accessing applications on a virtual machine](../network/virtualization/vm-publishing.html).
