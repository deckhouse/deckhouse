---
title: "Guest OS and bootloader of a virtual machine"
permalink: en/user/virtualization/vm-os-and-boot.html
description: "Guest operating system type and bootloader of a virtual machine: BIOS, UEFI, Secure Boot, and Windows boot specifics."
search: OS type, bootloader, UEFI, BIOS, Secure Boot, osType
---

The `osType` parameter defines the operating system type and applies the optimal set of virtual devices and parameters for the VM to work correctly.

Supported values:

- `Generic` (default): For Linux and other operating systems. The standard virtual device configuration is used.
- `Windows`: For Microsoft Windows operating systems. Automatically enables Hyper-V features, a TPM device, and other settings optimized for Windows.
- `Legacy`: For operating systems without built-in AHCI and virtio drivers: Windows XP, Windows 2000, Windows Server 2003, DOS-era systems, and Linux with a kernel older than 2.6.19. Such a VM gets the i440fx chipset, and with `enableParavirtualization: false` it also gets the IDE bus for disks and CD-ROM and the RTL8139 network adapter, whose drivers these operating systems have.

{% alert level="warning" %}
A virtual machine gets an emulated TPM whose state is kept in memory and isn't persisted. When the VM restarts or migrates, the TPM state is reset. Keep this limitation in mind when using Windows security features that depend on TPM.
{% endalert %}

The set of virtual devices the guest OS sees:

| Device                         | `Generic`                                                   | `Windows`                                                   | `Legacy`                     |
| ------------------------------ | ----------------------------------------------------------- | ----------------------------------------------------------- | ---------------------------- |
| Chipset                        | q35                                                         | q35                                                         | i440fx                       |
| Bootloader                     | `BIOS`, `EFI`, `EFIWithSecureBoot`                          | `BIOS`, `EFI`, `EFIWithSecureBoot`                          | `BIOS` only                  |
| Disk bus                       | virtio-scsi, SATA with `enableParavirtualization: false`    | virtio-scsi, SATA with `enableParavirtualization: false`    | virtio-blk, IDE with `enableParavirtualization: false` |
| CD-ROM bus                     | virtio-scsi, SATA with `enableParavirtualization: false`    | virtio-scsi, SATA with `enableParavirtualization: false`    | IDE |
| Block devices in [`.spec.blockDeviceRefs`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-blockdevicerefs) | up to 16                                          | up to 16                                                    | up to 16, up to 4 with `enableParavirtualization: false` |
| Network adapter                | virtio-net, e1000 with `enableParavirtualization: false`    | virtio-net, e1000 with `enableParavirtualization: false`    | virtio-net, RTL8139 with `enableParavirtualization: false` |
| USB controller                 | xHCI (USB 3.0)                                              | xHCI (USB 3.0)                                              | UHCI (USB 1.1)               |
| TPM                            | no                                                          | TPM 2.0                                                     | no                           |
| Random number generator        | virtio-rng                                                  | no                                                          | no                           |
| Hyper-V features               | no                                                          | yes                                                         | no                           |
| Attaching disks on the fly     | yes                                                         | yes                                                         | only with `enableParavirtualization: true` and only if the guest OS has the virtio-scsi driver |
| Changing CPU and memory on the fly | yes                                                     | yes                                                         | no                           |

USB device passthrough works for `Legacy`, but the UHCI controller is limited to the USB 1.1 speed of 12 Mbps, so fast storage in such a VM hits the bus limit.

Choose `Legacy` when the guest operating system can't work with an AHCI controller, not just because it's old. For Linux with kernel 2.6.19 and newer, `osType: Generic` with `enableParavirtualization: false` is a fit, because there's no four-device limit there and attaching disks on the fly through [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) remains available.

{% alert level="warning" %}
For `Legacy`, the following isn't available:

- Changing the number of CPU cores and the amount of memory on a running VM, because these guest operating systems don't bring them into service. The change is accepted, the VM shows it in [`.status.restartAwaitingChanges`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-status-restartawaitingchanges) along with the `AwaitingRestartToApplyConfiguration` condition, and it applies after a restart.
- Changing the contents of [`.spec.blockDeviceRefs`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-blockdevicerefs) on a running VM, because these disks stay static in both paravirtualization modes, so add a block device before the VM starts.
- The `EFI` and `EFIWithSecureBoot` bootloaders, as well as initialization (`cloud-init` and Sysprep), because these guest operating systems don't support them.
- Guest OS information in the VM status and file system information.

You can install the QEMU guest agent in such an OS from an archived virtio-win release, and the VM does show `AgentReady`, but its version is too old for the module. The VM gets the `AgentVersionNotSupported` condition, guest OS information isn't collected, and there's nothing to update the agent to.

A snapshot with `requiredConsistency: true` also doesn't complete successfully, but for a different reason. The module requests a file system freeze, and the agent replies that the command is disabled in its build with the `guest-fsfreeze-status has been disabled for this instance` message. On Windows, the freeze goes through the VSS provider, which isn't in this build. The snapshot waits in the `InProgress` phase for about ten minutes and moves to `Failed`, so set `requiredConsistency: false` for such VMs.

With `enableParavirtualization: false`, one more limitation applies. There can be no more than four block devices in total, because the IDE bus provides two channels with two devices each.
{% endalert %}

Example configuration for a Windows XP virtual machine:

```yaml
spec:
  osType: Legacy
  bootloader: BIOS
  enableParavirtualization: false
  # other parameters...
```

The `bootloader` parameter defines the bootloader type of the virtual machine:

- `BIOS` (default): Uses the legacy BIOS.
- `EFI`: Uses the Unified Extensible Firmware Interface (UEFI/EFI).
  - `EFIWithSecureBoot`: Uses UEFI/EFI with Secure Boot support.

Example configuration for a Windows virtual machine:

```yaml
spec:
  osType: Windows
  bootloader: EFI
  # other parameters...
```

Example configuration for a Linux virtual machine (you can omit the default values):

```yaml
spec:
  osType: Generic
  bootloader: BIOS
  # other parameters...
```

{% alert level="info" %}
For modern Linux distributions, choose `bootloader: EFI`, and for Windows, choose `bootloader: EFI` or `bootloader: EFIWithSecureBoot`.
{% endalert %}

{% alert level="warning" %}
`EFIWithSecureBoot` needs a persistent volume for the Secure Boot state, and creating it needs a default StorageClass in the cluster. If there's none, the virtual machine doesn't start and stays in the `Pending` state, and its status says that the default StorageClass isn't found. As soon as a default StorageClass appears, the machine starts automatically.
{% endalert %}

The `enableParavirtualization` parameter controls the use of the `virtio` bus for attaching the virtual devices of the VM. A change to this parameter takes effect only after the VM restarts.

- `true` (default): The `virtio` bus is used for disks, network interfaces, and other devices, which gives better performance. You can change the contents of [`.spec.blockDeviceRefs`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-blockdevicerefs) on a running VM without a restart, adding and removing devices, if the disk is available on the node where the VM runs.
- `false`: Standard device emulation is used (SATA for disks, e1000 for network interfaces; IDE and RTL8139 for the `Legacy` OS type), which may be required for compatibility with older operating systems that lack `VirtIO` drivers. Changes to [`.spec.blockDeviceRefs`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-blockdevicerefs) on a running VM (adding and removing disks and images, including ISO) take effect after the VM restarts. To attach and detach disks without a restart, use the [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) (`vmbda`) resource, without changing the list in the VM specification.

{% alert level="info" %}
To use the paravirtualization mode (`virtio`), some operating systems require the matching drivers to be installed. If the drivers aren't installed, the VM may fail to boot or the devices may work incorrectly.
{% endalert %}

For the `Legacy` OS type, the default value `true` isn't a fit, because these operating systems have no built-in virtio drivers, so for a VM you're going to install from the original media, set `enableParavirtualization: false`, otherwise the installer reports that it found no hard drives. A warning is issued when creating and modifying such a VM.

Keep `enableParavirtualization: true` for a VM with the `Legacy` OS type only when the virtio drivers are already installed in the guest OS. Then the VM keeps the i440fx chipset and the `BIOS` bootloader, but gets disks on virtio-blk, the virtio-net adapter, and loses the four-device limit. The CD-ROM stays on the IDE bus, because virtio-blk has no drive.

To switch a system that's already installed:

1. Install the virtio drivers in the guest OS. For Windows XP, 2000, and Server 2003, take an archived virtio-win release, because the current releases no longer contain drivers for these systems, and install `viostor`, the virtio-blk driver. The package has no virtio-scsi driver for these operating systems.
1. Power off the VM.
1. Set `enableParavirtualization: true`.
1. Start the VM.

The order of the steps matters. The disk controller driver has to appear in the guest OS before the switch, not after, because the VM boots from that very controller. If the VM doesn't boot after the switch, set `enableParavirtualization: false` back and restart it. The disks return to the IDE bus and the guest OS boots as before.

Example configuration with paravirtualization disabled:

```yaml
spec:
  enableParavirtualization: false
  # other parameters...
```
