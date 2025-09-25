---
title: "Overview"
permalink: en/virtualization-platform/documentation/admin/platform-management/storage/sds/node-configurator/about.html
---

Deckhouse Virtualization Platform (DVP) provides automatic management of logical volumes (Logical Volume Manager, LVM) on cluster nodes using custom Kubernetes resources. This functionality is provided by the `sds-node-configurator` module and includes:

- Detection of block devices on each node and creation of corresponding [BlockDevice](/modules/sds-node-configurator/stable/cr.html#blockdevice) resources.

  > Manual creation and modification of the [BlockDevice](/modules/sds-node-configurator/stable/cr.html#blockdevice) resource is prohibited.

- Automatic discovery and management of [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources with the label `storage.deckhouse.io/enabled=true` (including thin pools) on cluster nodes. When Volume Groups (VG) are detected on nodes without corresponding resources, the controller automatically creates the [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resource.

- Regular scanning of Physical Volumes (PV) that are part of managed Volume Groups (VG) on nodes. Upon expansion of the underlying block device, the controller performs `pvresize` on the corresponding physical volume and automatically increases the size of all logical volumes in this volume group.

  > Reducing the size of the block device is not supported.

- Synchronization of the [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) state with the actual state of the node: creation of new volume groups according to [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup), expansion of existing ones when `desiredCapacity` changes, and deletion of the group when the resource is deleted. For detailed examples of operation, see the section [Examples of working with LVMVolumeGroup](./usage.html#working-with-lvmvolumegroup-resources).
