---
title: "Release notes"
permalink: en/virtualization-platform/documentation/release-notes.html
---

## v1.0.0

### New features

* [vm] Added protection to prevent a cloud image (`VirtualImage` \ `ClusterVirtualImage`) from being connected as the first disk. Previously, this caused the VM to fail to start with the "No bootable device" error.
* [vmop] Added `Restore` operation to restore a VM from a previously created snapshot.

### Fixes

* [vmsnapshot] When restoring a virtual machine from a snapshot, all annotations and labels that were present on the resources at the time of the snapshot are now restored correctly.
* [module] Fixed an issue with queue blocking when the `settings.modules.publicClusterDomain` parameter was empty in the global ModuleConfig resource.
* [module] Optimized hook performance during module installation.
* [vmclass] Fixed core/coreFraction validation in the `VirtualMachineClass` resource.
* [module] When the SDN module is disabled, the configuration of additional networks in the VM is not available.

### Security

* Fixed CVE-2025-47907

## v0.25.0

### Important notes before update

In version v0.25.0, support for the module's operation with CRI containerd V2 has been added.
After upgrading CRI from containerd v1 to containerd v2, it is necessary to recreate the images that were created using virtualization module version v0.24.0 and earlier.

### New Features

- [observability] New Prometheus metrics have been added to track the phase of resources such as `VirtualMachineSnapshot`, `VirtualDiskSnapshot`, `VirtualImage`, and `ClusterVirtualImage`.
- [vm] MAC address management for additional network interfaces has been added using the `VirtualMachineMACAddress` and `VirtualMachineMACAddressLease` resources.
- [vm] Added the ability to attach additional network interfaces to a virtual machine for networks provided by the `SDN` module. For this, the `SDN` module must be enabled in the cluster.
- [vmclass] An annotation has been added to set the default `VirtualMachineClass`. You can designate a `VirtualMachineClass` as the default by adding the annotation
  `virtualmachineclass.virtualization.deckhouse.io/is-default-class=true`.
This allows creating VMs with an empty `spec.virtualMachineClassName` field, which will be automatically filled with the default class.

### Fixes

- [module] Added validation to ensure that virtual machine subnets do not overlap with system subnets (`podSubnetCIDR` and `serviceSubnetCIDR`).
- [vi] To create a virtual image on a `PersistentVolumeClaim`, the storage must support the `RWX` and `Block` modes; otherwise, a warning will be displayed.
- [vm] Fixed an issue where changing the operating system type caused the machine to enter a reboot loop.
- [vm] Fixed an issue where a virtual machine would hang in the Starting phase when project quotas were insufficient. A quota shortage message will now be displayed in the virtual machine's status. To allow the machine to continue starting, the project quotas need to be increased.

### Other

- [vm] Improved the garbage collector (GC) for completed virtual machine operations:
  - Runs daily at 00:00.
  - Removes successfully completed operations (`Completed` / `Failed`) after their TTL (24 hours) expires.
  - Retains only the last 10 completed operations.
