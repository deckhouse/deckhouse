---
title: "Release notes"
permalink: en/virtualization-platform/documentation/release-notes.html
---

## v1.7.0

<span style="opacity:0.6; font-style:italic; font-size:0.9em;">
Release date: March 31, 2026.
</span>

### New features

- [vm] The order of additional network interfaces is now deterministic and does not change after virtual machine restarts.
- [vm] Added a mechanism to prevent TCP connection drops during live migration of a virtual machine.
- [vm] Reduced USB device downtime during virtual machine migration.
- [vm] Added a garbage collector for completed and failed virtual machine pods:
  - Pods older than 24 hours are deleted.
  - No more than 2 completed pods are retained.
- [usb] When scheduling virtual machines on nodes, the system now takes into account whether a USB device uses USB 2.0 (High-Speed) or USB 3.0 (SuperSpeed).

### Fixes

- [vm] Fixed double storage quota consumption during migration of a virtual machine with local storage.
- [vm] When using [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) with the `Clone` or `Restore` type, disks now also restore their association with the virtual machine (owner reference).
- [vm] Fixed virtual machine eviction during node drain: pods responsible for block device attachments are no longer removed from a cordoned node before virtual machine migration is complete.
- [vm] Block devices can now be attached and detached even if the virtual machine is running on a cordoned node.
- [vm] Fixed validation for the `AlwaysForced` virtual machine migration policy: [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) resources with the `Evict` or `Migrate` type without explicit `force=true` are now rejected for this policy.
- [vm] Fixed an issue where a virtual machine could get stuck in the `Maintenance` state during restore from a snapshot.
- [vm] Added storage-side error messages (from the CSI driver) to the virtual machine status for block device attachment failures.
- [vd,vi,cvi] Fixed the creation of block devices from VMDK files (especially for VMDKs in the `streamOptimized` format used in exports from VMware).
- [usb] Stabilized USB device support for virtualization on Deckhouse Kubernetes Platform version `>=1.76` and Kubernetes version `>=1.33`.
- [usb] Fixed USB device detection on the host: duplicate USB devices could previously appear.

## v1.6.1

<span style="opacity:0.6; font-style:italic; font-size:0.9em;">
Release date: March 10, 2026.
</span>

### Fixes

- [observability] Restored the previous placement of virtual machine dashboards due to a validation issue that could block the Deckhouse queue.
- [vm] Fixed USB device discovery on nodes: corresponding [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) resources might not have been created.
- [vm] Fixed cloning of a virtual machine with connected USB devices when using [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) with the `Clone` type in `BestEffort` mode.

### Security

- [module] Fixed vulnerabilities CVE-2026-24051 and CVE-2025-15558.

## v1.6.0

<span style="opacity:0.6; font-style:italic; font-size:0.9em;">
Release date: March 2, 2026.
</span>

### New features

- [vm] Added support for attaching USB devices to virtual machines via `.spec.usbDevices`.
- [usb] Added [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) and [USBDevice](/modules/virtualization/cr.html#usbdevice) resources to manage USB devices in the cluster:
  - [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) (cluster-scoped): Represents a USB device discovered on a specific node. Allows assigning a USB device for use in a specific namespace.
  - [USBDevice](/modules/virtualization/cr.html#usbdevice) (namespace-scoped): Represents a USB device available for attachment to virtual machines in a given namespace.
- [observability] Added the `Virtualization / Overview` dashboard with an overview of the virtualization platform status.
- [observability] Added information about virtual machine pods to the virtual machine dashboard.
- [dvcr] Enabled DVCR cleanup in clusters by default: daily at 02:00. You can override the schedule via `dvcr.gc.schedule` in the `virtualization` module ModuleConfig.

### Fixes

- [vd] Fixed virtual disks hanging during creation in `WaitForFirstConsumer` mode on nodes with taints.
- [vm] If only the `Main` network is specified in `.spec.networks`, the `sdn` module is no longer required.
- [vm] Fixed virtual machine migration with disks attached via [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) (hotplug): the target pod could exceed memory limits (`OOMKilled`).
- [vmbda] Fixed an incorrect `Pending` phase for the [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) resource during virtual machine migration.
- [vmbda] To remove disks and images attached to a virtual machine via [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) (hotplug), you must first detach them from the virtual machine by deleting the corresponding `vmbda`. This information has been added to the `vmbda` status.

### Other

- [vm] Added the `--from-file` flag to the `vlctl` utility for viewing domain information from a local libvirt XML file.

## v1.5.2

<span style="opacity:0.6; font-style:italic; font-size:0.9em;">
Release date: March 5, 2026.
</span>

### Fixes

- [vd] Fixed a potential `OOMKill` during the virtual disk creation on NFS.

## v1.5.1

<span style="opacity:0.6; font-style:italic; font-size:0.9em;">
Release date: February 16, 2026.
</span>

### Fixes

- [vd] Fixed an issue with creating a virtual disk from a virtual image stored on a `PersistentVolumeClaim` (with `.spec.storage` set to `PersistentVolumeClaim`).

## v1.5.0

<span style="opacity:0.6; font-style:italic; font-size:0.9em;">
Release date: February 9, 2026.
</span>

### New features

- [vm] Added support for targeted migration of virtual machines.
  To do this, create a [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) resource with the `Migrate` type and specify `.spec.migrate.nodeSelector` to migrate the virtual machine to the corresponding node.
- [observability] Added a table with virtual machine operations to the `Namespace / Virtual Machine` dashboard.

### Fixes

- [core] Fixed an issue with starting virtual machines using the `EFIWithSecureBoot` bootloader when configured with more than 12 vCPUs.
- [vmop] Fixed an issue with cloning a virtual machine whose disks use storage in `WaitForFirstConsumer` mode.
- [module] System component resources required for starting and running virtual machines are no longer counted in project quotas.
- [module] During virtual machine migration, temporary double consumption of resources is no longer counted in project quotas.
- [module] Platform system components in user projects are protected from deletion by users.
- [vm] Fixed a possible virtual machine hang in the `Pending` state during migration when changing the StorageClass.
- [vd] Fixed an issue with live migration of a virtual machine between StorageClass with the `Filesystem` type.

### Other

- [vd] When viewing disks, the name of the virtual machine they are attached to is now displayed (`d8 k get vd`).

## v1.4.1

<span style="opacity:0.6; font-style:italic; font-size:0.9em;">
Release date: February 16, 2026.
</span>

### Security

- [module] Fixed vulnerabilities CVE-2025-61726, CVE-2025-61728, CVE-2025-61730, and CVE-2025-68121.

## v1.4.0

<span style="opacity:0.6; font-style:italic; font-size:0.9em;">
Release date: January 23, 2026.
</span>

### New features

- [vd] Added support for changing the StorageClass of disks attached via [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) (hotplug).
- [vd] Added support for migrating virtual machines with local disks attached via [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) (hotplug).
- [vm] Virtual machines can now be started without a `Main` network.

### Fixes

- [module] Fixed project quota accounting for resources used by system components required to create disks/images and operate virtual machines.
- [vi,cvi] Added tracking of image availability in DVCR. If an image disappears from DVCR, the corresponding [VirtualImage](/modules/virtualization/cr.html#virtualimage) and [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) resources enter the `Lost` phase and report an error.
- [vmip] Fixed IP address attachment when the corresponding [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) resource was created manually in advance.
- [vm] Added support for cloning virtual machines in the `Running` phase via [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) of type `Clone`.

## v1.3.0

<span style="opacity:0.6; font-style:italic; font-size:0.9em;">
Release date: December 16, 2025.
</span>

### New features

- [vmclass] Added the `.spec.sizingPolicies.defaultCoreFraction` field to the [VirtualMachineClass](/modules/virtualization/cr.html#virtualmachineclass) resource, allowing you to set the default `coreFraction` for virtual machines that use this class.

### Fixes

- [vi/cvi] Added the ability to use system nodes to create project and cluster images.
- [vd] Accelerated disk attachment in `WaitForFirstConsumer` mode for virtual machines.
- [vd] Fixed an issue with restoring labels and annotations on a disk created from a snapshot.
- [observability] Fixed the display of virtual machine charts in clusters running in HA mode.

## v1.2.2

<span style="opacity:0.6; font-style:italic; font-size:0.9em;">
Release date: December 5, 2025.
</span>

### Fixes

- [module] Fixed RBAC access permissions for the `d8:use:role:user` role that prevented it from managing the [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) resource.

## v1.2.1

<span style="opacity:0.6; font-style:italic; font-size:0.9em;">
Release date: December 4, 2025.
</span>

### Fixes

- [module] The deprecated part of the configuration has been removed, which could have prevented the virtualization module from upgrading in clusters running Kubernetes version 1.34 and above.

## v1.2.0

<span style="opacity:0.6; font-style:italic; font-size:0.9em;">
Release date: November 28, 2025.
</span>

### New features

- [vmrestore] The [VirtualMachineRestore](/modules/virtualization/cr.html#virtualmachinerestore) resource is deprecated. Use the following resources instead:
  - [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) with type `Clone`: For cloning an existing virtual machine.
  - [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) with type `Restore`: For restoring an existing virtual machine to a state from a snapshot.
  - [VirtualMachineSnapshotOperation](/modules/virtualization/cr.html#virtualmachinesnapshotoperation): For creating a new virtual machine based on a snapshot.
- [vmsop] Added the [VirtualMachineSnapshotOperation](/modules/virtualization/cr.html#virtualmachinesnapshotoperation) resource for creating a virtual machine based on a [VirtualMachineSnapshot](/modules/virtualization/cr.html#virtualmachinesnapshot).
- [vmclass] For the [VirtualMachineClass](/modules/virtualization/cr.html#virtualmachineclass) resource, version `v1alpha2` is deprecated. Use version `v1alpha3` instead:
  - In version `v1alpha3`, the `.spec.sizingPolicies.coreFraction` field is now a string with a percentage (for example, "50%"), similar to the field in a virtual machine.
- [module] Added validation for the virtualization ModuleConfig that prevents decreasing the DVCR storage size and changing its StorageClass.
- [module] Improved audit events by using more informative messages that include virtual machine names and user information.
- [module] Added the ability to clean up DVCR from non-existent project and cluster images:
  - By default, this feature is disabled.
  - To enable cleanup, set a schedule in the module settings: `.spec.settings.dvcr.gc.schedule`.
- [vmbda] Added detailed error output in the `Attached` condition of the [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) resource when a block device is unavailable on the virtual machine node.
- [module] Added new metrics for disks:
  - `d8_virtualization_virtualdisk_capacity_bytes`: Metric showing the disk size.
  - `d8_virtualization_virtualdisk_info`: Metric with information about the disk configuration.
  - `d8_virtualization_virtualdisk_status_inuse`: Metric showing the current use of the disk by a virtual machine or for creating other block devices.

### Fixes

- [vmclass] Added the ability to modify or delete the [VirtualMachineClass](/modules/virtualization/cr.html#virtualmachineclass) resource named "generic". The virtualization module will no longer restore it to its original state.
- [vm] Fixed the `MethodNotAllowed` error for `patch` and `watch` operations when querying the [VirtualMachineClass](/modules/virtualization/cr.html#virtualmachineclass) resource via command-line utilities (`d8 k`, `kubectl`).
- [image] Fixed an issue that prevented deleting [VirtualImage](/modules/virtualization/cr.html#virtualimage) and [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) resources for a stopped virtual machine.
- [module] Fixed RBAC for the `user` and `editor` cluster roles.
- [module] Fixed the `D8VirtualizationVirtualMachineFirmwareOutOfDate` alert, which could be duplicated when virtualization runs in HA mode.
- [snapshot] Fixed an error that could lead to inconsistencies between [VirtualMachineSnapshot](/modules/virtualization/cr.html#virtualmachinesnapshot) and [VirtualDiskSnapshot](/modules/virtualization/cr.html#virtualdisksnapshot) resources when creating a snapshot of a virtual machine with multiple disks.

### Security

- [module] Fixed vulnerability CVE-2025-64324.

## v1.1.3

<span style="opacity:0.6; font-style:italic; font-size:0.9em;">
Release date: November 21, 2025.
</span>

### Security

- [module] Fixed CVE-2025-64324, CVE-2025-64435, CVE-2025-64436, CVE-2025-58183, CVE-2025-58186, CVE-2025-58187, CVE-2025-58188, CVE-2025-52565, CVE-2025-52881, CVE-2025-31133.

### Other

- [observability] The virtual machine overview dashboards (`Namespace / Virtual Machine` and `Namespace / Virtual Machines`) have been improved: in addition to the cluster level, they are now also available at the project level.

## v1.1.2

<span style="opacity:0.6; font-style:italic; font-size:0.9em;">
Release date: November 5, 2025.
</span>

### Fixes

- [vd] Fixed live disk migration between StorageClasses that use different drivers. Restrictions:
  - Migration between `Block` and `Filesystem` is not supported. Only migrations between the same volume mode are allowed: `Block` → `Block` and `Filesystem` → `Filesystem`.
- [vm] In the `Migrating` state, detailed error information is now displayed when a live migration of a virtual machine fails.

## v1.1.1

<span style="opacity:0.6; font-style:italic; font-size:0.9em;">
Release date: October 16, 2025.
</span>

### Fixes

- [core] Fixed an issue in the containerd v2 where storage providing a PVC with the `Filesystem` type was incorrectly attached via [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment).
- [core] Added error reporting in the status of disks and images when the data source (URL) is unavailable.
- [vi] When creating virtual images from virtual disk snapshots, the `.spec.persistentVolumeClaim.storageClassName` parameter is now respected. Previously, it could be ignored.
- [vm] Fixed the `NetworkReady` condition output: it no longer shows the `Unknown` state and appears only when needed.
- [vm] Prohibited duplicate networks in the virtual machine `.spec.network` specification.
- [vmip] Added validation for static IP addresses to avoid creating a [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) resource with an IP already in use in the cluster.
- [vmbda] Fixed a bug where, when detaching a virtual image through [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment), the resource could get stuck in the `Terminating` state.

### Other

- [observability] Added Prometheus metrics for virtual machine snapshots (`d8_virtualization_virtualmachinesnapshot_info`) and virtual disk snapshots (`d8_virtualization_virtualdisksnapshot_info`), showing which objects they are associated with.

### Security

- [module] Fixed vulnerabilities CVE-2025-58058 and CVE-2025-54410.

## v1.1.0

<span style="opacity:0.6; font-style:italic; font-size:0.9em;">
Release date: October 6, 2025.
</span>

### New features

- [vm] Added the ability to migrate VMs using disks on local storage. Restrictions:
  - The feature is not available in the CE edition.
  - Migration is only possible for running VMs (`phase: Running`).
  - Migration of VMs with local disks connected via [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) (hotplug) is not supported yet.
- [vd] Added the ability to migrate storage for VM disks (change StorageClass). Restrictions:
  - The feature is not available in the CE edition.
  - Migration is only possible for running VMs (`phase: Running`).
  - Storage migration for disks connected via [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) (hotplug) is not supported yet.
- [vmop] Added an operation with the `Clone` type to create a clone of a VM from an existing VM ([VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) `.spec.type: Clone`).
- [observability] Added the `KubeNodeAwaitingVirtualMachinesEvictionBeforeShutdown` alert, which is triggered when the node hosting the virtual machines is about to shut down but VM evacuation is not yet complete.
- [observability] Added the `D8VirtualizationDVCRInsufficientCapacityRisk` alert, which warns of the risk of insufficient free space in the virtual machine image storage (DVCR).

### Fixes

- [vmclass] Fixed an issue in [VirtualMachineClass](/modules/virtualization/cr.html#virtualmachineclass) types `Features` and `Discovery` that caused nested virtualization not to work on nodes with AMD processors.
- [vmop/restore] Fixed a bug where the controller sometimes started a restored VM before its disks were fully restored, resulting in the VM starting with old (unrestored) disks.
- [vmsnapshot] Fixed behavior when creating a VM snapshot with uncommitted changes: the snapshot now instantly captures the current state of the virtual machine, including all current changes.
- [module] Fixed an issue with installing the module on RedOS 8.X OS.
- [module] Improved validation to prevent adding empty values for parameters that define StorageClass for disks and images.
- [vmop] Fixed garbage collector behavior: previously, all VMOP objects were deleted after restarting the virtualization controller, ignoring cleanup rules.
- [observability] The virtual machine dashboard now displays statistics for all networks (including additional ones) connected to the VM.
- [observability] Fixed the graph on the virtual machine dashboard that displays memory copy statistics during VM migration.

## v1.0.0

<span style="opacity:0.6; font-style:italic; font-size:0.9em;">
Release date: September 11, 2025.
</span>

### New features

- [vm] Added protection to prevent a cloud image ([VirtualImage](/modules/virtualization/cr.html#virtualimage) \ [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage)) from being connected as the first disk. Previously, this caused the VM to fail to start with the "No bootable device" error.
- [vmop] Added `Restore` operation to restore a VM from a previously created snapshot.

### Fixes

- [vmsnapshot] When restoring a virtual machine from a snapshot, all annotations and labels that were present on the resources at the time of the snapshot are now restored correctly.
- [module] Fixed an issue with queue blocking when the `settings.modules.publicClusterDomain` parameter was empty in the global ModuleConfig resource.
- [module] Optimized hook performance during module installation.
- [vmclass] Fixed `core`/`coreFraction` validation in the [VirtualMachineClass](/modules/virtualization/cr.html#virtualmachineclass) resource.
- [module] When the `sdn` module is disabled, the configuration of additional networks in the VM is not available.

### Security

- Fixed CVE-2025-47907.

## v0.25.0

<span style="opacity:0.6; font-style:italic; font-size:0.9em;">
Release date: August 29, 2025.
</span>

### Important notes before update

In version v0.25.0, support for the module's operation with CRI containerd v2 has been added.
After upgrading CRI from containerd v1 to containerd v2, it is necessary to recreate the images that were created using the virtualization module version v0.24.0 or earlier.

### New features

- [observability] New Prometheus metrics have been added to track the phase of resources such as [VirtualMachineSnapshot](/modules/virtualization/cr.html#virtualmachinesnapshot), [VirtualDiskSnapshot](/modules/virtualization/cr.html#virtualdisksnapshot), [VirtualImage](/modules/virtualization/cr.html#virtualimage), and [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage).
- [vm] MAC address management for additional network interfaces has been added using the [VirtualMachineMACAddress](/modules/virtualization/cr.html#virtualmachinemacaddress) and [VirtualMachineMACAddressLease](/modules/virtualization/cr.html#virtualmachinemacaddresslease) resources.
- [vm] Added the ability to attach additional network interfaces to a virtual machine for networks provided by the `sdn` module. For this, the `sdn` module must be enabled in the cluster.
- [vmclass] An annotation has been added to set the default `VirtualMachineClass`. You can designate a [VirtualMachineClass](/modules/virtualization/cr.html#virtualmachineclass) as the default by adding the annotation
  `virtualmachineclass.virtualization.deckhouse.io/is-default-class=true`.
  This allows creating VMs with an empty `spec.virtualMachineClassName` field, which will be automatically filled with the default class.

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
