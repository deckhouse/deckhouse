---
title: "Release notes"
permalink: en/virtualization-platform/documentation/release-notes.html
---

## v1.1.1

### Fixes

[core] Fixed an issue in the containerd v2 where storage providing a PVC with the FileSystem type was incorrectly attached via `VirtualMachineBlockDeviceAttachment`.
- [core] Added error reporting in the status of disks and images when the data source (URL) is unavailable.
- [vi] When creating virtual images from virtual disk snapshots, the `spec.persistentVolumeClaim.storageClassName` parameter is now respected. Previously, it could be ignored.
- [vm] Fixed the `NetworkReady` condition output: it no longer shows the `Unknown` state and appears only when needed.
- [vm] Prohibit duplicate networks in the virtual machine `.spec.network` specification.
- [vmip] Added validation for static IP addresses to avoid creating a `VirtualMachineIPAddress` resource with an IP already in use in the cluster.
- [vmbda] Fixed a bug where, when detaching a virtual image through `VirtualMachineBlockDeviceAttachment`, the resource could get stuck in the Terminating state.

### Other

- [observability] Added Prometheus metrics for virtual machine snapshots (`d8_virtualization_virtualmachinesnapshot_info`) and virtual disk snapshots (`d8_virtualization_virtualdisksnapshot_info`), showing which objects they are associated with.

### Security

- [module] Fixed vulnerabilities CVE-2025-58058 and CVE-2025-54410.

## v1.1.0

### New features

- [vm] Added the ability to migrate VMs using disks on local storage. Restrictions:
  - The feature is not available in the CE edition.
  - Migration is only possible for running VMs (`phase: Running`).
  - Migration of VMs with local disks connected via `VirtualMachineBlockDeviceAttachment` (hotplug) is not supported yet.
- [vd] Added the ability to migrate storage for VM disks (change `StorageClass`). Restrictions:
  - The feature is not available in the CE edition.
  - Migration is only possible for running VMs (`phase: Running`).
  - Storage migration for disks connected via `VirtualMachineBlockDeviceAttachment` (hotplug) is not supported yet.
- [vmop] Added an operation with the `Clone` type to create a clone of a VM from an existing VM (`VirtualMachineOperation` `.spec.type: Clone`).
- [observability] Added the `KubeNodeAwaitingVirtualMachinesEvictionBeforeShutdown` alert, which is triggered when the node hosting the virtual machines is about to shut down but VM evacuation is not yet complete.
- [observability] Added the `D8VirtualizationDVCRInsufficientCapacityRisk` alert, which warns of the risk of insufficient free space in the virtual machine image storage (DVCR).

### Fixes

- [vmclass] Fixed an issue in `VirtualMachineClass` types Features and Discovery that caused nested virtualization not to work on nodes with AMD processors.
- [vmop/restore] Fixed a bug where the controller sometimes started a restored VM before its disks were fully restored, resulting in the VM starting with old (unrestored) disks.
- [vmsnapshot] Fixed behavior when creating a VM snapshot with uncommitted changes: the snapshot now instantly captures the current state of the virtual machine, including all current changes.
- [module] Fixed an issue with installing the `virtualization` module on RedOS 8.X OS.
- [module] Improved validation to prevent adding empty values for parameters that define storage classes for disks and images.
- [vmop] Fixed garbage collector behavior: previously, all VMOP objects were deleted after restarting the virtualization controller, ignoring cleanup rules.
- [observability] The virtual machine dashboard now displays statistics for all networks (including additional ones) connected to the VM.
- [observability] Fixed the graph on the virtual machine dashboard that displays memory copy statistics during VM migration.

## v1.0.0

### New features

- [vm] Added protection to prevent a cloud image (`VirtualImage` \ `ClusterVirtualImage`) from being connected as the first disk. Previously, this caused the VM to fail to start with the "No bootable device" error.
- [vmop] Added `Restore` operation to restore a VM from a previously created snapshot.

### Fixes

- [vmsnapshot] When restoring a virtual machine from a snapshot, all annotations and labels that were present on the resources at the time of the snapshot are now restored correctly.
- [module] Fixed an issue with queue blocking when the `settings.modules.publicClusterDomain` parameter was empty in the global `ModuleConfig` resource.
- [module] Optimized hook performance during `virtualization` module installation.
- [vmclass] Fixed core/coreFraction validation in the `VirtualMachineClass` resource.
- [module] When the `SDN` module is disabled, the configuration of additional networks in the VM is not available.

### Security

- Fixed CVE-2025-47907.

## v0.25.0

### Important notes before update

In version v0.25.0, support for the `virtualization` module's operation with CRI containerd v2 has been added.
After upgrading CRI from containerd v1 to containerd v2, it is necessary to recreate the images that were created using `virtualization` module version v0.24.0 and earlier.

### New Features

- [observability] New Prometheus metrics have been added to track the phase of resources such as `VirtualMachineSnapshot`, `VirtualDiskSnapshot`, `VirtualImage`, and `ClusterVirtualImage`.
- [vm] MAC address management for additional network interfaces has been added using the `VirtualMachineMACAddress` and `VirtualMachineMACAddressLease` resources.
- [vm] Added the ability to attach additional network interfaces to a virtual machine for networks provided by the `SDN` module. For this, the `SDN` module must be enabled in the cluster.
- [vmclass] An annotation has been added to set the default `VirtualMachineClass`. You can designate a `VirtualMachineClass` as the default by adding the annotation
  `virtualmachineclass.virtualization.deckhouse.io/is-default-class=true`. This allows creating VMs with an empty `spec.virtualMachineClassName` field, which will be automatically filled with the default class.

### Fixes

- [module] Added validation to ensure that virtual machine subnets do not overlap with system subnets (`podSubnetCIDR` and `serviceSubnetCIDR`).
- [vi] To create a virtual image on a `PersistentVolumeClaim`, the storage must support the `RWX` and `Block` modes; otherwise, a warning will be displayed.
- [vm] Fixed an issue where changing the operating system type caused the machine to enter a reboot loop.
- [vm] Fixed an issue where a virtual machine would hang in the `Starting` phase when project quotas were insufficient. A quota shortage message will now be displayed in the virtual machine's status. To allow the machine to continue starting, the project quotas need to be increased.

### Other

- [vmop] Improved the garbage collector (GC) for completed virtual machine operations:
  - Runs daily at 00:00.
  - Removes successfully completed operations (`Completed` / `Failed`) after their TTL (24 hours) expires.
  - Retains only the last 10 completed operations.
