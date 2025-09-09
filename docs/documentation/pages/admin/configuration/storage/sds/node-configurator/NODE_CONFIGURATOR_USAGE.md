---
title: "Usage"
permalink: en/admin/configuration/storage/sds/node-configurator/usage.html
---

{% alert level="info" %}
Functionality is guaranteed only when using stock kernels supplied with [supported distributions](../../../../../supported_versions.html#linux). When using non-standard kernels or distributions, behavior may be unpredictable.
{% endalert %}

The controller operates with two types of Kubernetes custom resources:

- [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice): A resource representing a block device.
- [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup): A resource describing a Logical Volume Manager volume group.

## Working with BlockDevice resources

### Creating a BlockDevice resource

The controller periodically scans available block devices on each node and automatically creates a [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice) resource when it detects a device that matches the rules. As a result, a resource object with a unique name is created, containing detailed information about the device's characteristics.

#### Criteria for device selection by the controller

- The device is not used as a DRBD volume (Distributed Replicated Block Device).
- The device is not a loop interface (virtual block device).
- The device is a logical volume.
- The file system is absent or has the label `LVM2_MEMBER` (indicating it belongs to an LVM device).
- The device does not contain partitions (no partition table).
- The device's capacity exceeds 1 GiB.
- For virtual disks, a serial number is required.

The created [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice) resource serves as a data source for subsequent work with [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resources.

### Updating a BlockDevice resource

When the state of a block device changes (for example, size change, metadata change, or the device disappears from the system), the controller automatically detects these changes and updates the fields of the [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice) resource, ensuring the information is up-to-date. User edits to the [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice) resource are prohibited and will be overwritten by the controller.

### Deleting a BlockDevice resource

The controller deletes a [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice) resource only when the following conditions are met:

- The resource is in the `Consumable` state (available for consumption in the Logical Volume Manager).
- The block device is no longer available in the system (the device has been removed or disconnected).
- The block device is not part of a volume group with the label `storage.deckhouse.io/enabled=true` (such a volume group is managed by [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup), and the [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice) is not deleted).

{% alert level="info" %}
A [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice) resource is deleted without a help from a user. If the user manually deletes the resource, the controller will recreate it during the next scan if the device still meets the criteria.
{% endalert %}

## Working with LVMVolumeGroup resources

The [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource combines several [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice) resources into a single Logical Volume Manager (LVM) volume group on a node and reflects the current state of this group.

{% alert level="info" %}
Currently, only the `Local` type (local volume groups) is supported.
{% endalert %}

### Creating an LVMVolumeGroup resource

There are two scenarios for creating an [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource:

- Automatic creation of the resource:
  - The controller scans the list of active LVM volume groups on each node.
  - If a discovered volume group has the label `storage.deckhouse.io/enabled=true` and there is no corresponding [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource, the controller automatically creates the resource.
  - The controller fills out the `spec` section, except for the `thinPools` field. All other parameters (block device names, Volume Group name, etc.) are automatically pulled from the system state.
  - To manage thin pools, the user can manually add information about them to the `spec` section after the resource is created.

- Manual creation of the resource:
  - The user creates a YAML manifest, specifying the minimum set of fields:
    - `metadata.name`: Unique name for the [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource.
    - `spec`: Name of the node where the volume group will exist.
  - After validation, the controller creates the volume group on the node and updates the custom resource with current information about the state of the created LVM Volume Group.

  Example of creating a local Volume Group without a thin pool:

  ```yaml
  apiVersion: storage.deckhouse.io/v1alpha1
  kind: LVMVolumeGroup
  metadata:
    name: "vg-0-on-node-0"
  spec:
    type: Local
    local:
      nodeName: "node-0"
    blockDeviceSelector:
      matchExpressions:
        - key: kubernetes.io/metadata.name
          operator: In
          values:
            - dev-07ad52cef2348996b72db262011f1b5f896bb68f
            - dev-e90e8915902bd6c371e59f89254c0fd644126다7
    actualVGNameOnTheNode: "vg-0"
  ```

  ```yaml
  apiVersion: storage.deckhouse.io/v1alpha1
  kind: LVMVolumeGroup
  metadata:
    name: "vg-0-on-node-0"
  spec:
    type: Local
    local:
      nodeName: "node-0"
    blockDeviceSelector:
      matchLabels:
        kubernetes.io/hostname: node-0
    actualVGNameOnTheNode: "vg-0"
  ```

  Example of creating a local Volume Group with a thin pool (250 GiB):

  ```yaml
  apiVersion: storage.deckhouse.io/v1alpha1
  kind: LVMVolumeGroup
  metadata:
    name: "vg-0-on-node-0"
  spec:
    type: Local
    local:
      nodeName: "node-0"
    blockDeviceSelector:
      matchExpressions:
        - key: kubernetes.io/metadata.name
          operator: In
          values:
            - dev-07ad52cef2348996b72db262011f1b5f896bb68f
            - dev-e90e8915902bd6c371e59f89254c0fd644126다7
    actualVGNameOnTheNode: "vg-0"
    thinPools:
      - name: thin-1
        size: 250Gi
  ```

  ```yaml
  apiVersion: storage.deckhouse.io/v1alpha1
  kind: LVMVolumeGroup
  metadata:
    name: "vg-0-on-node-0"
  spec:
    type: Local
    local:
      nodeName: "node-0"
    blockDeviceSelector:
      matchLabels:
        kubernetes.io/hostname: node-0
    actualVGNameOnTheNode: "vg-0"
    thinPools:
      - name: thin-1
        size: 250Gi
  ```

The user can use any valid selectors (`matchLabels` or `matchExpressions`) to select [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice) resources:

- `matchLabels` allows selecting all devices that have the specified label (for example, `kubernetes.io/hostname=node-0`).
- `matchExpressions` allows for more flexible expressions, such as including listed device names.

The `spec.local.nodeName` field must match the name of the node where the Volume Group is created. Otherwise, the resource will not be started.
All selected block devices must physically reside on the same node for a resource of type Local.

{% alert level="warning" %}
After applying the manifest, the controller will automatically create or update the LVM infrastructure on the node, including working with physical volumes (`pvcreate`), creating or modifying the volume group (`vgcreate`/`vgextend`), and setting up the thin pool (`lvcreate`).
{% endalert %}

### Updating an LVMVolumeGroup resource

To change the configuration of the volume group (adding/removing block devices or changing thin pool parameters), it is sufficient to edit the `spec` section of the resource. The controller will validate the new parameters and adjust the state on the node (execute commands like `vgextend`, `lvcreate`, `lvresize`, etc.).

The `status` section of the resource automatically reflects the current information about the state of the Volume Group (list of physical volumes, thin pool data, free/used space, etc.).

{% alert level="warning" %}

- The controller does not change the `spec` automatically — it only reads the desired state and brings the actual state of the node into compliance. All changes to the desired state must be made by the user in `spec`.
- Avoid modifying `status` manually, as this section is managed by the controller.

{% endalert %}

### Deleting an LVMVolumeGroup resource

The controller automatically deletes the resource if the Volume Group no longer exists on the node (for example, all block devices are disconnected or removed). For manual deletion of the Volume Group and its associated logical volumes, the user can use:

```shell
d8 k delete lvg <resource-name>
```

After that, the controller will perform a full removal of the physical and logical LVM objects (`lvremove`, `vgremove`, etc.).

{% alert level="warning" %}
If there are logical volumes (including thin pools) in the Volume Group, the user must first delete or move them (using commands `lvremove` or `pvmove` + `vgreduce`). Otherwise, the controller will not be able to delete the resource.
To prevent accidental deletion of the resource, you can set the annotation `storage.deckhouse.io/deletion-protection` — while it is present, the controller will not initiate the deletion of the volume group.
{% endalert %}

### Removing a BlockDevice from an LVMVolumeGroup

1. To exclude a specific [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice) from the Volume Group, adjust the `spec.blockDeviceSelector` selector of the [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource or remove the corresponding label from the [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice) resource.

1. On the node, manually transfer data (if there are volumes in the Volume Group) using the following command:

   ```shell
   pvmove <source-device>
   ```

1. Remove the physical volume from the group:

   ```shell
   vgreduce <VG-name> <excluded-device>
   ```

1. Finally, remove the physical volume metadata:

   ```shell
   pvremove <excluded-device>
   ```

{% alert level="info" %}
If logical volumes remain during the removal process, delete them in advance using the `lvremove` command.
To protect against unintentional deletion, you can use the annotation `storage.deckhouse.io/deletion-protection` — until it is removed, the [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource will not be deleted.
{% endalert %}

## Protecting against data leaks between volumes

When files are deleted, the operating system does not erase the data on the disk: the corresponding blocks are merely marked as "free". If the same physical blocks are used when creating a new volume, they may contain residual data from the previous user.

For example:

- User #1 created a volume from StorageClass #1 on node #1 (regardless of the "Block" or "Filesystem" mode) and wrote files to it.
- User #1 deleted the files and the volume itself.
- The blocks that the volume occupied were marked as free but not overwritten.
- User #2 requested a new volume from StorageClass #1 on the same node #1 in block mode.
- There is a risk that some or all of the previously freed blocks will be allocated to user #2 again.
- As a result, user #2 may gain access to user #1's data.

### Thick volume

To ensure confidentiality, the `volumeCleanup` parameter is used in the `PersistentVolume` configuration. Available modes:

- Parameter not set: When the volume is deleted, no additional operations are performed, the blocks remain uninitialized, and the data may be accessible to the next user.
- `RandomFillSinglePass`: The volume is overwritten once with random data (it is recommended to avoid this on solid-state drives due to wear).
- `RandomFillThreePass`: The volume is overwritten three times with random data (maximum reliability but high load on the device).
- `Discard`: Using TRIM, all block devices are marked as free. Effective on most modern solid-state drives, but depends on the SSD controller's support for DRAT/RZAT.

#### Using Discard mode

Most modern solid-state drives support the `discard` (TRIM) command, which marks freed blocks as "zero" and does not return old data on subsequent reads. Therefore, using the `Discard` option is the most reliable way to prevent data leaks on solid-state drives. However, the following nuances should be considered:

- The `discard` command initiates cell cleaning, but the actual zeroing is performed by the drive's controller in the background. Until this operation is completed, some blocks may still contain old data.
- Not all solid-state drives clean specific physical blocks individually: many devices work at the page level and larger blocks, so individual logical blocks may remain uncleaned until the entire page is fully processed.
- To be sure that after `discard` the blocks are indeed read as zeros, the disk must support DRAT (Deterministic TRIM) — predictable behavior of the `discard` command and RZAT (Deterministic Read Zero after TRIM) — guaranteed reading of "zeros" after `discard`. If the device has not confirmed support for DRAT and RZAT, there is no guarantee that the freed blocks will never return the old data.
- Even with declared support for DRAT/RZAT, manufacturers may not fully comply with the specifications. In the absence of independent tests or certification of solid-state drives, it is recommended to refrain from using the `Discard` mode for critical tasks related to data security.

{% alert level="warning" %}
If the drive is not confirmed to support DRAT and RZAT (meaning, has not passed verification for deterministic cleaning and reading of zeroed blocks), the use of the `Discard` option is **not recommended**, as there remains a risk of recovering old data.
{% endalert %}

### Thin volume

When a block of a thin volume is released, the `discard` command from the guest OS is passed directly to the storage device. If a mechanical hard drive or a solid-state drive without `discard` support is used, the freed blocks in the thin pool remain with the previous data until they are physically overwritten. However, clients are provided only with thin volumes, not access to the thin pool itself. When a block is next allocated to a thin-LV, LVM guaranteedly fills the corresponding block in the thin pool with zeros (zero-fill), thanks to the `thin_pool_zero=1` parameter. This ensures the absence of residual data between different thin volumes and prevents information leaks between clients.
