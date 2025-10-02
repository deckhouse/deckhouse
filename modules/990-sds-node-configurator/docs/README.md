---
title: "The sds-node-configurator module"
description: "General Concepts and Principles of the sds-node-configurator module. Deckhouse Kubernetes Platform."
---

{% alert level="warning" %}
The module is guaranteed to work only with stock kernels that are shipped with the [supported distributions](https://deckhouse.io/documentation/v1/supported_versions.html#linux).

The module may work with other kernels or distributions, but its stable operation and availability of all features is not guaranteed.
{% endalert %}

The module manages `LVM` on cluster nodes through [Kubernetes custom resources](./cr.html) by performing the following operations:

  - Discovering block devices and creating/updating/deleting their corresponding [BlockDevice resources](./cr.html#blockdevice).

   > **Caution!** Manual creation and modification of the `BlockDevice` resource is prohibited.

  - Discovering `LVM Volume Groups` on the nodes with the `storage.deckhouse.io/enabled=true` LVM tag attached and `Thin-pools` running on them as well as managing the corresponding [LVMVolumeGroup resources](./cr.html#lvmvolumegroup). The module automatically creates an `LVMVolumeGroup` resource if it does not yet exist for a discovered `LVM Volume Group`.

  - Scanning `LVM Physical Volumes` on the nodes that are part of managed `LVM Volume Groups`. In case the size of underlying block device expands, the corresponding `LVM Physical Volumes` will be automatically expanded as well (`pvresize` will occur).

  > **Caution!** Downsizing a block device is not supported.

  - Creating/expanding/deleting `LVM Volume Groups` on the node according to the changes the user has made to the `LVMVolumeGroup` resources. [Usage examples](./usage.html#lvmvolumegroup-resources)
