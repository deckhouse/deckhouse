---
title: "Setting up local storage based on LVM"
permalink: en/admin/configuration/storage/sds/lvm-local.html
description: "Configure local LVM-based storage in Deckhouse Kubernetes Platform. High-performance local storage setup for test environments and EDGE clusters with reduced network latency."
---

Local storage reduces network latency and provides higher performance compared to remote storage accessed over a network. This approach is particularly effective in test environments and EDGE clusters. This functionality is provided by the [`sds-local-volume`](/modules/sds-local-volume/) module.

## Configuring local storage

To ensure the correct operation of the `sds-local-volume` module, follow these steps:

1. Configure LVMVolumeGroup. Before creating a StorageClass, you must create an [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource for the `sds-node-configurator` module on the cluster nodes.
1. Enable the [`sds-node-configurator`](/modules/sds-node-configurator/) module. Ensure that the module is enabled **before** enabling the `sds-local-volume` module.
1. Create the corresponding StorageClasses. Creating a StorageClass for the CSI driver `local.csi.storage.deckhouse.io` by a user is **prohibited**.

The module supports two operating modes: LVM and LVMThin.

## Quick start

All commands are executed on a machine with access to the Kubernetes API and administrator privileges.

### Enabling modules

Enabling the [`sds-node-configurator`](/modules/sds-node-configurator/) module:

1. Create a ModuleConfig resource to enable the module:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: sds-node-configurator
   spec:
     enabled: true
     version: 1
   EOF
   ```

1. Wait for the module to reach the `Ready` state. At this stage, it is not necessary to check the pods in the `d8-sds-node-configurator` namespace.

   ```shell
   d8 k get modules sds-node-configurator -w
   ```

Enabling the [`sds-local-volume`](/modules/sds-local-volume/) module:

1. Activate the `sds-local-volume` module. The example below starts the module with default settings, which will create service pods for the `sds-local-volume` component on all cluster nodes:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: sds-local-volume
   spec:
     enabled: true
     version: 1
   EOF
   ```

1. Wait for the module to reach the `Ready` state.

   ```shell
   d8 k get modules sds-local-volume -w
   ```

1. Ensure that all pods in the `d8-sds-local-volume` and `d8-sds-node-configurator` namespaces are in the `Running` or `Completed` state and are running on all nodes where LVM resources are planned to be used.

   ```shell
   d8 k -n d8-sds-local-volume get pod -owide -w
   d8 k -n d8-sds-node-configurator get pod -o wide -w
   ```

### Preparing nodes for storage creation

For storage to function correctly on nodes, the `sds-local-volume-csi-node` pods must be running on the selected nodes.

By default, these pods are launched on all cluster nodes. You can verify their presence using the command:

```shell
d8 k -n d8-sds-local-volume get pod -owide
```

The placement of `sds-local-volume-csi-node` pods is managed by specific labels (`nodeSelector`). These labels are set in the [`spec.settings.dataNodes.nodeSelector`](/modules/sds-local-volume/configuration.html#parameters-datanodes-nodeselector) parameter of the module.

### Configuring storage on nodes

To configure storage on nodes, you need to create LVM volume groups using [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resources. This example creates a Thick storage.

{% alert level="warning" %}
Before creating an [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource, ensure that the `sds-local-volume-csi-node` pod is running on the respective node. This can be checked with the command:

```shell
d8 k -n d8-sds-local-volume get pod -owide
```

{% endalert %}

1. Retrieve all [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice) resources available in your cluster:

   ```shell
   d8 k get bd
   ```

   Example output:

   ```console
   NAME                                           NODE       CONSUMABLE   SIZE           PATH
   dev-ef4fb06b63d2c05fb6ee83008b55e486aa1161aa   worker-0   false        976762584Ki    /dev/nvme1n1
   dev-0cfc0d07f353598e329d34f3821bed992c1ffbcd   worker-0   false        894006140416   /dev/nvme0n1p6
   dev-7e4df1ddf2a1b05a79f9481cdf56d29891a9f9d0   worker-1   false        976762584Ki    /dev/nvme1n1
   dev-b103062f879a2349a9c5f054e0366594568de68d   worker-1   false        894006140416   /dev/nvme0n1p6
   dev-53d904f18b912187ac82de29af06a34d9ae23199   worker-2   false        976762584Ki    /dev/nvme1n1
   dev-6c5abbd549100834c6b1668c8f89fb97872ee2b1   worker-2   false        894006140416   /dev/nvme0n1p6
   ```

1. Create an [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource for the `worker-0` node:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: LVMVolumeGroup
   metadata:
     name: "vg-1-on-worker-0" # The name can be any fully qualified resource name in Kubernetes. This LVMVolumeGroup resource name will be used to create LocalStorageClass in the future.
   spec:
     type: Local
     local:
       nodeName: "worker-0"
     blockDeviceSelector:
       matchExpressions:
         - key: kubernetes.io/metadata.name
           operator: In
           values:
             - dev-ef4fb06b63d2c05fb6ee83008b55e486aa1161aa
             - dev-0cfc0d07f353598e329d34f3821bed992c1ffbcd
     actualVGNameOnTheNode: "vg-1" # The name of the LVM VG that will be created from the specified block devices on the node.
   EOF
   ```

1. Wait for the created [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource to transition to the `Ready` state:

   ```shell
   d8 k get lvg vg-1-on-worker-0 -w
   ```

   If the resource has transitioned to the `Ready` state, it means that an LVM VG named `vg-1` has been created on the `worker-0` node from the block devices `/dev/nvme1n1` and `/dev/nvme0n1p6`.

1. Create an [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource for the `worker-1` node:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: LVMVolumeGroup
   metadata:
     name: "vg-1-on-worker-1"
   spec:
     type: Local
     local:
       nodeName: "worker-1"
     blockDeviceSelector:
       matchExpressions:
         - key: kubernetes.io/metadata.name
           operator: In
           values:
             - dev-7e4df1ddf2a1b05a79f9481cdf56d29891a9f9d0
             - dev-b103062f879a2349a9c5f054e0366594568de68d
     actualVGNameOnTheNode: "vg-1"
   EOF
   ```

1. Wait for the created [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource to transition to the `Ready` state:

   ```shell
   d8 k get lvg vg-1-on-worker-1 -w
   ```

   If the resource has transitioned to the `Ready` state, it means that an LVM VG named `vg-1` has been created on the `worker-1` node from the block devices `/dev/nvme1n1` and `/dev/nvme0n1p6`.

1. Create an [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource for the `worker-2` node:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: LVMVolumeGroup
   metadata:
     name: "vg-1-on-worker-2"
   spec:
     type: Local
     local:
       nodeName: "worker-2"
     blockDeviceSelector:
       matchExpressions:
         - key: kubernetes.io/metadata.name
           operator: In
           values:
             - dev-53d904f18b912187ac82de29af06a34d9ae23199
             - dev-6c5abbd549100834c6b1668c8f89fb97872ee2b1
     actualVGNameOnTheNode: "vg-1"
   EOF
   ```

1. Wait for the created [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource to transition to the `Ready` state:

   ```shell
   d8 k get lvg vg-1-on-worker-2 -w
   ```

   If the resource has transitioned to the `Ready` state, it means that an LVM VG named `vg-1` has been created on the `worker-2` node from the block devices `/dev/nvme1n1` and `/dev/nvme0n1p6`.

1. Create a [LocalStorageClass](/modules/sds-local-volume/cr.html#localstorageclass) resource:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: LocalStorageClass
   metadata:
     name: local-storage-class
   spec:
     lvm:
       lvmVolumeGroups:
         - name: vg-1-on-worker-0
         - name: vg-1-on-worker-1
         - name: vg-1-on-worker-2
       type: Thick
     reclaimPolicy: Delete
     volumeBindingMode: WaitForFirstConsumer
   EOF
   ```

   For a thin volume:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: LocalStorageClass
   metadata:
     name: local-storage-class
   spec:
     lvm:
       lvmVolumeGroups:
         - name: vg-1-on-worker-0
         - name: vg-1-on-worker-1
         - name: vg-1-on-worker-2
           thin:
             poolName: thin-1
       type: Thin
     reclaimPolicy: Delete
     volumeBindingMode: WaitForFirstConsumer
   EOF
   ```

   > **Important.** In a [LocalStorageClass](/modules/sds-local-volume/cr.html#localstorageclass) with `type: Thick`, you cannot use an LVMVolumeGroup that contains at least one thin pool.

1. Wait for the created LocalStorageClass resource to transition to the `Created` state:

   ```shell
   d8 k get lsc local-storage-class -w
   ```

1. Verify that the corresponding StorageClass has been created:

   ```shell
   d8 k get sc local-storage-class
   ```

If a StorageClass named `local-storage-class` appears, the configuration of the [`sds-local-volume`](/modules/sds-local-volume/) module is complete. Users can now create PVCs by specifying the StorageClass named `local-storage-class`.

### Selecting a volume cleanup method after PV deletion

When files are deleted, the operating system does not physically erase the content but only marks the corresponding blocks as "free." If a new volume receives physical blocks previously used by another volume, those blocks may still contain the previous user's data.

This is possible, for example, in the following scenario:

- User #1 places files in a volume requested from StorageClass 1 on node 1 (in either "Block" or "Filesystem" mode).
- User #1 deletes the files and the volume.
- The physical blocks that were occupied become "free" but are not wiped.
- User #2 requests a new volume from StorageClass 1 on node 1 in "Block" mode.
- There is a risk that some or all of the blocks previously used by User #1 will be reallocated to User #2.
- In this case, User #2 may be able to recover User #1's data.

### Thick volumes

To prevent data leaks through thick volumes, the [`volumeCleanup`](/modules/sds-node-configurator/cr.html#lvmlogicalvolume-v1alpha1-spec-volumecleanup) parameter is provided. It allows you to select the volume cleanup method before deleting a PV.

Possible values:

- Parameter not set: No additional actions are performed when deleting the volume. Data may remain accessible to the next user.
- `RandomFillSinglePass`: The volume is overwritten with random data once before deletion. This option is not recommended for solid-state drives, as it reduces the drive's lifespan.
- `RandomFillThreePass`: The volume is overwritten with random data three times before deletion. This option is not recommended for solid-state drives, as it reduces the drive's lifespan.
- `Discard`: All volume blocks are marked as free using the `discard` system call before deletion. This option is only meaningful for solid-state drives.

  Most modern solid-state drives guarantee that a block marked with `discard` will not return previous data when read. This makes the `Discard` option the most effective way to prevent data leaks when using solid-state drives.

  However, erasing a cell is a relatively slow operation, so it is performed by the device in the background. Additionally, many drives cannot erase individual cells but only groups (pages). As a result, not all drives guarantee the immediate unavailability of freed data. Furthermore, not all drives that claim to guarantee this keep their promise. If a device does not guarantee Deterministic TRIM (DRAT), Deterministic Read Zero after TRIM (RZAT), and is not verified, it is not recommended for use.

### Thin volumes

When a block in a thin volume is freed via `discard` by the guest operating system, this command is forwarded to the device. If a hard disk is used or if the solid-state drive does not support `discard`, the data may remain in the thin pool until the block is reused. However, users are only granted access to thin volumes, not the thin pool itself. They can only obtain a volume from the pool, and for thin volumes, the thin pool block is zeroed upon reuse, preventing data leaks between clients. This is ensured by the `thin_pool_zero=1` setting in LVM.

## System requirements and recommendations

- Use stock kernels provided with [supported distributions](/products/kubernetes-platform/documentation/v1/reference/supported_versions.html#linux).
- Do not use another SDS (Software Defined Storage) to provide disks for SDS Deckhouse.
