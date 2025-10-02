---
title: "The sds-node-configurator module: FAQ"
description: "Deckhouse Kubernetes Platform. The sds-node-configurator module. Common questions and answers."
---

{% alert level="warning" %}
The module is guaranteed to work only with stock kernels that are shipped with the [supported distributions](https://deckhouse.io/documentation/v1/supported_versions.html#linux).

The module may work with other kernels or distributions, but its stable operation and availability of all features is not guaranteed.
{% endalert %}

## Why does creating BlockDevice and LVMVolumeGroup resources in a cluster fail?

- In most cases, the creation of BlockDevice resources fails because the existing devices fail filtering by the controller. Make sure that your devices meet the [requirements](./usage.html#the-conditions-the-controller-imposes-on-the-device).

- Creating LVMVolumeGroup resources may fail due to the absence of BlockDevice resources in the cluster, as their names are used in the LVMVolumeGroup specification.

- If the BlockDevice resources are present and the LVMVolumeGroup resources are not, make sure the existing `LVM Volume Group` on the node has the special tag `storage.deckhouse.io/enabled=true` attached.

## I have deleted the LVMVolumeGroup resource, but the resource and its `Volume Group` are still there. What do I do?

Such a situation is possible in two cases:

1. The `Volume Group` contains `LV`.

   The controller does not take responsibility for removing LV from the node, so if there are any logical volumes in the `Volume Group` created by the resource, you need to manually delete them on the node. After this, both the resource and the `Volume Group` (along with the `PV`) will be deleted automatically.

2. The resource has an annotation `storage.deckhouse.io/deletion-protection`.

   This annotation protects the resource from deletion and, as a result, the `Volume Group` created by it. You need to remove the annotation manually with the command:

   ```shell
   kubectl annotate lvg %lvg-name% storage.deckhouse.io/deletion-protection-
   ```

   After the command is executed, both the LVMVolumeGroup resource and `Volume Group` will be deleted automatically.

## I'm trying to create a `Volume Group` using the LVMVolumeGroup resource, but I'm not getting anywhere. Why?

Most likely, your resource fails controller validation even if it has passed the Kubernetes validation successfully.
The exact cause of the failure can be found in the `status.message` field of the resource.
You can also refer to the controller's logs.

The problem usually stems from incorrectly-defined BlockDevice resources. Make sure these resources meet the following requirements:

- The `Consumable` field is set to `true`.
- For a `Volume Group` of the `Local` type, the specified BlockDevice resources belong to the same node.<!-- > - For a `Volume Group` of the `Shared` type, the specified BlockDevice is the only resource. -->
- The current names of the BlockDevice resources are specified.

A full list of expected values can be found in the [CR reference](./cr.html) of the LVMVolumeGroup resource.

## What happens if I unplug one of the devices in a `Volume Group`? Will the linked LVMVolumeGroup resource be deleted?

The LVMVolumeGroup resource will persist as long as the corresponding `Volume Group` exists. As long as at least one device exists, the `Volume Group` will be there, albeit in an unhealthy state.
Note that these issues will be reflected in the resource's `status`.

Once the unplugged device is plugged back in and reactivated, the `LVM Volume Group` will regain its functionality while the corresponding LVMVolumeGroup resource will also be updated to reflect the current state.

## How to transfer control of an existing `LVM Volume Group` on the node to the controller?

Add the LVM tag `storage.deckhouse.io/enabled=true` to the LVM Volume Group on the node:

```shell
vgchange myvg-0 --addtag storage.deckhouse.io/enabled=true
```

## How do I get the controller to stop monitoring the `LVM Volume Group` on the node?

Delete the `storage.deckhouse.io/enabled=true` LVM tag for the target `Volume Group` on the node:

```shell
vgchange myvg-0 --deltag storage.deckhouse.io/enabled=true
```

The controller will then stop tracking the selected `Volume Group` and delete the associated LVMVolumeGroup resource automatically.

## I haven't added the `storage.deckhouse.io/enabled=true` LVM tag to the `Volume Group`, but it is there. How is this possible?

This can happen if you created the `LVM Volume Group` using the LVMVolumeGroup resource, in which case the controller will automatically add this LVM tag to the created `LVM Volume Group`. This is also possible if the `Volume Group` or its `Thin-pool` already had the `linstor-*` LVM tag of the `linstor` module.

When you switch from the `linstor` module to the `sds-node-configurator` and `sds-replicated-volume` modules, the `linstor-*` LVM tags are automatically replaced with the `storage.deckhouse.io/enabled=true` LVM tag in the `Volume Group`. This way, the `sds-node-configurator` gains control over these `Volume Groups`.

## How to use the LVMVolumeGroupSet resource to create LVMVolumeGroup?

To create an LVMVolumeGroup using the LVMVolumeGroupSet resource, you need to specify node selectors and a template for the LVMVolumeGroup resources in the LVMVolumeGroupSet specification. Currently, only the `PerNode` strategy is supported. With this strategy, the controller will create one LVMVolumeGroup resource from the template for each node that matches the selector.

Example of an LVMVolumeGroupSet specification:

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
      actualVGNameOnTheNode: <actual-vg-name-on-the-node>
```

## How to use the LVMVolumeGroupSet resource to create LVMVolumeGroup?

To create an LVMVolumeGroup using the LVMVolumeGroupSet resource, you need to specify node selectors and a template for the LVMVolumeGroup resources in the LVMVolumeGroupSet specification. Currently, only the `PerNode` strategy is supported. With this strategy, the controller will create one LVMVolumeGroup resource from the template for each node that matches the selector.

Example of an LVMVolumeGroupSet specification:

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
      actualVGNameOnTheNode: <actual-vg-name-on-the-node>
```

## Which labels are added by the controller to BlockDevice resources

* status.blockdevice.storage.deckhouse.io/type - LVM type

* status.blockdevice.storage.deckhouse.io/fstype - filesystem type

* status.blockdevice.storage.deckhouse.io/pvuuid - PV UUID

* status.blockdevice.storage.deckhouse.io/vguuid - VG UUID

* status.blockdevice.storage.deckhouse.io/partuuid - partition UUID

* status.blockdevice.storage.deckhouse.io/lvmvolumegroupname - resource name

* status.blockdevice.storage.deckhouse.io/actualvgnameonthenode - actual VG name on the node

* status.blockdevice.storage.deckhouse.io/wwn - WWN (World Wide Name) identifier for the device

* status.blockdevice.storage.deckhouse.io/serial - device serial number

* status.blockdevice.storage.deckhouse.io/size - size

* status.blockdevice.storage.deckhouse.io/model - device model

* status.blockdevice.storage.deckhouse.io/rota - whether it is a rotational device

* status.blockdevice.storage.deckhouse.io/hotplug - hot-plug capability

* status.blockdevice.storage.deckhouse.io/machineid - ID of the server on which the block device is installed
