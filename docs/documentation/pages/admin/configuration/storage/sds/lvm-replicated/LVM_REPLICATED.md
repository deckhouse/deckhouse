---
title: "Setting up replicated storage based on DRBD"
permalink: en/admin/configuration/storage/sds/lvm-replicated.html
description: "Configure replicated storage based on DRBD in Deckhouse Kubernetes Platform. High availability storage with LINSTOR backend for fault tolerance and data replication across nodes."
---

Data replication across multiple nodes ensures fault tolerance and data availability, even if a hardware or software failure occurs on one of the nodes. This guarantees data preservation on other nodes, maintaining continuous access. Such a model is essential for critical data and distributed infrastructures requiring high availability and minimizing data loss during failures.

To create replicated block StorageClass objects based on Distributed Replicated Block Device (DRBD), the [`sds-replicated-volume`](/modules/sds-replicated-volume/) module is used. It leverages [LINSTOR](https://linbit.com/linstor/) as the backend.

## Enabling the module

### Discovery of LVM components

Before creating StorageClass objects based on Logical Volume Manager (LVM), it is necessary to detect the block devices and volume groups available on the nodes and obtain current information about their state. To do this, enable the [`sds-node-configurator`](/modules/sds-node-configurator/) module:

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

Wait for the [`sds-node-configurator`](/modules/sds-node-configurator/) module to reach the `Ready` status. To check the status, run the following command:

```shell
d8 k get modules sds-node-configurator -w
```

In the output, you should see information about the [`sds-node-configurator`](/modules/sds-node-configurator/) module:

```console
NAME                       STAGE   SOURCE    PHASE       ENABLED    READY
sds-node-configurator              Embedded  Available   True       True
```

### Connecting DRBD

To enable the [`sds-replicated-volume`](/modules/sds-replicated-volume/) module with default settings, run the following command:

```shell
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: sds-replicated-volume
spec:
  enabled: true
  version: 1
EOF
```

This will install the DRBD kernel module on all cluster nodes, register the CSI driver, and launch the [`sds-replicated-volume`](/modules/sds-replicated-volume/) component Pods.

Wait until the [`sds-replicated-volume`](/modules/sds-replicated-volume/) module reaches the `Ready` status. To check the module status, run the following command:

```shell
d8 k get modules sds-replicated-volume -w
```

In the output, you should see information about the [`sds-replicated-volume`](/modules/sds-replicated-volume/) module:

```console
NAME                       STAGE   SOURCE    PHASE       ENABLED    READY
sds-replicated-volume              Embedded  Available   True       True
```

To check that all Pods in the `d8-sds-replicated-volume` and `d8-sds-node-configurator` namespaces are in the `Running` or `Completed` state and have been started on all nodes where DRBD resources are planned to be used, use the following commands:

```shell
d8 k -n d8-sds-replicated-volume get pod -w
d8 k -n d8-sds-node-configurator get pod -w
```

{% alert level="info" %}
Avoid configuring the `LINSTOR` backend manually, as this can can result in errors.
{% endalert %}

## Node pre-configuration

### Creating LVM volume groups

Before configuring the creation of StorageClass objects, you need to combine the available block devices on the nodes into LVM volume groups. These volume groups will later be used to place PersistentVolumes. To get the available block devices, you can use the [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice) resource, which reflects their current state:

```shell
d8 k get bd
```

In the output, you should see a list of available block devices:

```console
NAME                                           NODE       CONSUMABLE   SIZE           PATH
dev-ef4fb06b63d2c05fb6ee83008b55e486aa1161aa   worker-0   false        976762584Ki    /dev/nvme1n1
dev-0cfc0d07f353598e329d34f3821bed992c1ffbcd   worker-0   false        894006140416   /dev/nvme0n1p6
dev-7e4df1ddf2a1b05a79f9481cdf56d29891a9f9d0   worker-1   false        976762584Ki    /dev/nvme1n1
dev-b103062f879a2349a9c5f054e0366594568de68d   worker-1   false        894006140416   /dev/nvme0n1p6
dev-53d904f18b912187ac82de29af06a34d9ae23199   worker-2   false        976762584Ki    /dev/nvme1n1
dev-6c5abbd549100834c6b1668c8f89fb97872ee2b1   worker-2   false        894006140416   /dev/nvme0n1p6
```

In the example output, there are six block devices available across three nodes.

To combine the block devices on a node, you need to create an LVM volume group using the [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource. To create the LVMVolumeGroup resource on node `worker-0`, apply the following resource, replacing the node and block device names with your own:

```shell
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: LVMVolumeGroup
metadata:
  name: "vg-on-worker-0"
spec:
  type: Local
  local:
    # Replace with the name of the node where you are creating the volume group.
    nodeName: "worker-0"
  blockDeviceSelector:
    matchExpressions:
      - key: kubernetes.io/metadata.name
        operator: In
        values:
          # Replace with the names of the block devices for the node you are creating the volume group on.
          - dev-ef4fb06b63d2c05fb6ee83008b55e486aa1161aa
          - dev-0cfc0d07f353598e329d34f3821bed992c1ffbcd
  # The name of the LVM volume group that will be created from the specified block devices on the selected node.
  actualVGNameOnTheNode: "vg"
  # Uncomment if you want to be able to create thin pools.
  # thinPools:
  #   - name: thin-pool-0
  #     size: 70%
EOF
```

Wait for the created [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource to reach the `Ready` phase. To check the resource phase, run the following command:

```shell
d8 k get lvg vg-on-worker-0 -w
```

In the output, you should see information about the resource phase:

```console
NAME             THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE       ALLOCATED SIZE   VG   AGE
vg-on-worker-0   1/1         True                    Ready   worker-0   360484Mi   30064Mi          vg   1h
```

If the resource has transitioned to the `Ready` phase, this means that an LVM volume group named vg has been created on node `worker-0` from the block devices `/dev/nvme1n1` and `/dev/nvme0n1p6`.

Next, you need to repeat the creation of [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resources for the remaining nodes (`worker-1` and `worker-2`), changing the resource name, node name, and block device names accordingly.

Ensure that LVM volume groups have been created on all nodes where they are planned to be used by running the following command:

```shell
d8 k get lvg -w
```

In the output, you should see a list of created volume groups:

```console
NAME             THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE       ALLOCATED SIZE   VG   AGE
vg-on-worker-0   0/0         True                    Ready   worker-0   360484Mi   30064Mi          vg   1h
vg-on-worker-1   0/0         True                    Ready   worker-1   360484Mi   30064Mi          vg   1h
vg-on-worker-2   0/0         True                    Ready   worker-2   360484Mi   30064Mi          vg   1h
```

### Creating replicated thick pools

Now that the necessary LVM volume groups are created on the nodes, you need to combine them into a single logical space. This can be done by combining them into replicated storage pools in the `LINSTOR` backend through the [ReplicatedStoragePool](/modules/sds-replicated-volume/cr.html#replicatedstoragepool) resource interface.

Storage pools can be of two types: LVM (thick) and LVMThin (thin). The thick pool offers high performance, comparable to the performance of the storage device, but it does not allow snapshots. Example of creating a replicated thick pool:

```shell
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: ReplicatedStoragePool
metadata:
  name: thick-pool
spec:
  type: LVM
  lvmVolumeGroups:
    - name: vg-1-on-worker-0
    - name: vg-1-on-worker-1
    - name: vg-1-on-worker-2
EOF
```

Wait for the created [ReplicatedStoragePool](/modules/sds-replicated-volume/cr.html#replicatedstoragepool) resource to reach the `Completed` phase. To check the resource phase, run the following command:

```shell
d8 k get rsp data -w
```

In the output, you should see information about the resource phase:

```console
NAME         PHASE       TYPE   AGE
thick-pool   Completed   LVM    87d
```

### Creating replicated thin pools

Unlike thick pools, a thin pool allows the use of snapshots but has lower performance.

The previously created [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) is suitable for creating thick pools. If you need the ability to create replicated thin pools, update the configuration of the [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resources by adding a definition for the thin pool:

```shell
d8 k patch lvg vg-on-worker-0 --type='json' -p='[
  {
    "op": "add",
    "path": "/spec/thinPools",
    "value": [
      {
        "name": "thin-pool-0",
        "size": "70%"
      }
    ]
  }
]'
```

In the updated version of [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup), 70% of the available space will be used to create thin pools. The remaining 30% can be used for thick pools.

Repeat the addition of thin pools for the remaining nodes (`worker-1` and `worker-2`). Example of creating a replicated thin pool:

```shell
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: ReplicatedStoragePool
metadata:
  name: thin-pool
spec:
  type: LVMThin
  lvmVolumeGroups:
    - name: vg-1-on-worker-0
      thinPoolName: thin-pool-0
    - name: vg-1-on-worker-1
      thinPoolName: thin-pool-0
    - name: vg-1-on-worker-2
      thinPoolName: thin-pool-0
EOF
```

Wait for the created [ReplicatedStoragePool](/modules/sds-replicated-volume/cr.html#replicatedstoragepool) resource to transition to the `Completed` phase. To check the resource phase, run the following command:

```shell
d8 k get rsp data -w
```

In the output, you should see information about the resource phase:

```console
NAME        PHASE       TYPE      AGE
thin-pool   Completed   LVMThin   87d
```

## Creating StorageClass objects

StorageClass objects are created through the [ReplicatedStorageClass](/modules/sds-replicated-volume/cr.html#replicatedstorageclass) resource, which defines the configuration for the desired StorageClass. Manually creating a StorageClass resource without [ReplicatedStorageClass](/modules/sds-replicated-volume/cr.html#replicatedstorageclass) may lead to undesired behavior.

Example of creating a [ReplicatedStorageClass](/modules/sds-replicated-volume/cr.html#replicatedstorageclass) resource based on a thick pool, where the PersistentVolumes will be placed on volume groups across three nodes:

```shell
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: ReplicatedStorageClass
metadata:
  name: replicated-storage-class
spec:
  # Specify the name of one of the storage pools created earlier.
  storagePool: thick-pool
  # Reclaim policy when deleting PVC.
  # Allowed values: "Delete", "Retain".
  # [Learn more...](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#reclaiming)
  reclaimPolicy: Delete
  # Replicas can be placed on any available node: no more than one replica of a specific volume on one node.
  # The cluster does not have zones (no nodes with the topology.kubernetes.io/zone labels).
  topology: Ignored
  # Replication mode where the volume remains available for read and write even if one replica becomes unavailable.
  # Data is stored in three instances across different nodes.
  replication: ConsistencyAndAvailability
EOF
```

Check that the created [ReplicatedStorageClass](/modules/sds-replicated-volume/cr.html#replicatedstorageclass) resource has transitioned to the `Created` phase by running the following command:

```shell
d8 k get rsc replicated-storage-class -w
```

In the output, you should see information about the created [ReplicatedStorageClass](/modules/sds-replicated-volume/cr.html#replicatedstorageclass) resource:

```console
NAME                       PHASE     AGE
replicated-storage-class   Created   1h
```

Check that the corresponding StorageClass has been generated by running the following command:

```shell
d8 k get sc replicated-storage-class
```

In the output, you should see information about the generated StorageClass:

```console
NAME                       PROVISIONER                      RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
replicated-storage-class   local.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   1h
```

If a StorageClass with the name `replicated-storage-class` appears, it means the configuration of the [`sds-replicated-volume`](/modules/sds-replicated-volume/) module is complete. Users can now create PersistentVolume objects by specifying the `replicated-storage-class` StorageClass. With the above configuration, a volume with three replicas across different nodes will be created.
