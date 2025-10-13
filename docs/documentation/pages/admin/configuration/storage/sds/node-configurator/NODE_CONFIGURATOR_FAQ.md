---
title: "FAQ"
permalink: en/admin/configuration/storage/sds/node-configurator/faq.html
---

{% alert level="info" %}
Functionality is guaranteed only when using stock kernels supplied with [supported distributions](../../../../../supported_versions.html#linux). When using non-standard kernels or distributions, behavior may be unpredictable.
{% endalert %}

## Reasons why BlockDevice resources aren't created in the cluster

Most often, [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice) resources are not created because the available devices do not pass the controller's filters. Ensure that the devices meet the [selection criteria](./usage.html#criteria-for-device-selection-by-the-controller).

## Reasons why LVMVolumeGroup resources aren't created in the cluster

- Absence of [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice): The controller will not create an [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) if there are no BlockDevice resources in the cluster specified in its specification.
- Absence of a tag: If [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice) resources are present but the [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) is missing, check that the corresponding LVM group on the node has the tag `storage.deckhouse.io/enabled=true`.

## Reasons why after deleting the LVMVolumeGroup, the resource and Volume Group remain

This situation is possible in two cases:

1. There are logical volumes in the Volume Group — the controller is not responsible for deleting logical volumes on the node, so if there are any logical volumes in the Volume Group created via the resource, they must be manually deleted. After that, both the [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource and the Volume Group itself (including physical volumes) will be automatically deleted.

1. The resource has the annotation `storage.deckhouse.io/deletion-protection` — this annotation protects the resource from deletion, along with the associated Volume Group. Remove the annotation with the command:

   ```shell
   d8 k annotate lvg <resource-name> storage.deckhouse.io/deletion-protection-
   ```

   After that, the [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource and the corresponding Volume Group will be automatically deleted.

## Reasons for failed attempts to create a Volume Group using the LVMVolumeGroup resource

Most likely, the resource did not pass validation by the controller (unlike the Kubernetes schema). The reason can be found in the `status.message` field of the [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource or in the controller logs.
Check that the specified [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice) resources meet the conditions:

- The `consumable` field is set to `true`.
- For `spec.type: Local`, all [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice) resources belong to the same node.
- Current names of [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice) resources are used.

## Behavior of the LVMVolumeGroup resource when one of the devices in the Volume Group is disabled

The [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource remains as long as the corresponding LVM group exists. When a device becomes unavailable, the group enters an error state, which is reflected in the `status` field.

After the device is restored, the group returns to the `Healthy` status, and the resource status is automatically updated.

## Transferring management of an existing Volume Group on the node to the controller

Add the tag `storage.deckhouse.io/enabled=true` to the desired Volume Group:

```shell
vgchange myvg-0 --addtag storage.deckhouse.io/enabled=true
```

The controller will create the corresponding [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource and take over management of the group.

## Disabling tracking of the LVM Volume Group by the controller

Remove the tag `storage.deckhouse.io/enabled=true`:

```shell
vgchange myvg-0 --deltag storage.deckhouse.io/enabled=true
```

The controller will stop tracking and delete the associated [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource.

## Reasons for automatic setting of the tag storage.deckhouse.io/enabled=true on the Volume Group

The controller adds the tag when creating the Volume Group through the [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource.

When migrating from the `linstor` module to [`sds-node-configurator`](/modules/sds-node-configurator/) and [`sds-replicated-volume`](/modules/sds-replicated-volume/), all `linstor-*` tags are replaced with `storage.deckhouse.io/enabled=true` to transfer management to the new logic.

## Using the LVMVolumeGroupSet resource to create LVMVolumeGroup

The [LVMVolumeGroupSet](/modules/sds-node-configurator/cr.html#lvmvolumegroupset) resource allows template-based creation of [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) on nodes. Currently, the PerNode strategy is supported — one resource per node that matches the selector.

Example specification of [LVMVolumeGroupSet](/modules/sds-node-configurator/cr.html#lvmvolumegroupset):

```yaml
apiVersion: storage.deckhouse.io/v1alpha1
kind: LVMVolumeGroupSet
metadata:
  name: my-lvm-volume-group-set
  labels:
    my-label: my-value
spec:
  strategy: PerNode
  nodeSelector:
    matchLabels:
      node-role.kubernetes.io/worker: ""
  lvmVolumeGroupTemplate:
    metadata:
      labels:
        my-label-for-lvg: my-value-for-lvg
    spec:
      type: Local
      blockDeviceSelector:
        matchLabels:
          status.blockdevice.storage.deckhouse.io/model: <model>
      actualVGNameOnTheNode: <VG-name-on-the-node>
```

## Labels added by the controller to BlockDevice resources

- `status.blockdevice.storage.deckhouse.io/type`: LVM type.
- `status.blockdevice.storage.deckhouse.io/fstype`: File system type.
- `status.blockdevice.storage.deckhouse.io/pvuuid`: Physical volume (PV) UUID.
- `status.blockdevice.storage.deckhouse.io/vguuid`: Volume group (VG) UUID.
- `status.blockdevice.storage.deckhouse.io/partuuid`: Partition UUID.
- `status.blockdevice.storage.deckhouse.io/lvmvolumegroupname`: Name of the [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource.
- `status.blockdevice.storage.deckhouse.io/actualvgnameonthenode`: Name of the LVM Volume Group on the node.
- `status.blockdevice.storage.deckhouse.io/wwn`: WWN (World Wide Name) of the device.
- `status.blockdevice.storage.deckhouse.io/serial`: Serial number of the device.
- `status.blockdevice.storage.deckhouse.io/size`: Size of the device.
- `status.blockdevice.storage.deckhouse.io/model`: Model of the device.
- `status.blockdevice.storage.deckhouse.io/rota`: Rotational device flag.
- `status.blockdevice.storage.deckhouse.io/hotplug`: Hotplug capability.
- `status.blockdevice.storage.deckhouse.io/machineid`: Identifier of the machine where the device is installed.
