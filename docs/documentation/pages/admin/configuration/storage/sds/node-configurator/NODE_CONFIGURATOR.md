---
title: "Overview"
permalink: en/admin/configuration/storage/sds/node-configurator/about.html
---

Deckhouse Kubernetes Platform provides automatic management of logical volumes (Logical Volume Manager, LVM) on cluster nodes using custom Kubernetes resources. This functionality is provided by the `sds-node-configurator` module and includes:

- Detection of block devices on each node and creation of corresponding [BlockDevice](../../../../../reference/cr/blockdevices/) resources.

{% alert level="warning" %}
Manual creation and modification of the [BlockDevice](../../../../../reference/cr/blockdevices/) resource is prohibited.
{% endalert %}

- Automatic discovery and management of [LVMVolumeGroup](../../../../../reference/cr/lvmvolumegroup/) resources with the label `storage.deckhouse.io/enabled=true` (including thin pools) on cluster nodes. When Volume Groups (VG) are detected on nodes without corresponding resources, the controller automatically creates the [LVMVolumeGroup](../../../../../reference/cr/lvmvolumegroup/) resource.

- Regular scanning of Physical Volumes (PV) that are part of managed Volume Groups (VG) on nodes. Upon expansion of the underlying block device, the controller performs `pvresize` on the corresponding physical volume and automatically increases the size of all logical volumes in this volume group.

{% alert level="warning" %}
Reducing the size of the block device is not supported.
{% endalert %}

Synchronization of the [LVMVolumeGroup](../../../../../reference/cr/lvmvolumegroup/) state with the actual state of the node: creation of new volume groups according to [LVMVolumeGroup](../../../../../reference/cr/lvmvolumegroup/), expansion of existing ones when `desiredCapacity` changes, and deletion of the group when the resource is deleted. For detailed examples of operation, see the section [Examples of working with LVMVolumeGroup](./usage.html#working-with-lvmvolumegroup-resources).
