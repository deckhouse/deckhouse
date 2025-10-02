---
title: "Module sds-node-configurator: sds-module configuration scenarios"
linkTitle: "Configuration scenarios"
description: "Sds-module configuration scenarios using sds-node-configurator"
---

{% alert level="warning" %}
The module's functionality is guaranteed only when using stock kernels provided with [supported distributions](https://deckhouse.io/documentation/v1/supported_versions.html#linux).

The module may work with other kernels or distributions, but this is not guaranteed.
{% endalert %}

{% alert level="info" %}
If you create virtual machines by cloning, you must change the UUID of the volume groups (VG) on the cloned VMs. To do this, run the `vgchange -u` command. This will generate new UUID for all VG on the virtual machine. You can add this to the `cloud-init` script if needed.

You can only change the UUID of a VG if it has no active logical volumes (LV). To deactivate a logical volume, unmount it and run the following command:

```shell
lvchange -an <VG_or_LV_NAME>
```

, where `<VG_or_LV_NAME>` — the name of a VG, to deactivate all LV in the VG, or the name of a LV, to deactivate a specific LV.
{% endalert %}

## Configuration methods and scenarios for node disk subsystems

On this page, you can find two methods for configuring the disk subsystem on Kubernetes cluster nodes,
depending on storage conditions:

- [Storage with identical disks](#storage-with-identical-disks)
- [Combined storage](#combined-storage)

Each configuration method comes with two configuration scenarios:

- "Full mirror". We recommend using this scenario due to its simplicity and reliability.
- "Partial mirror".

The following table contains details, advantages, and disadvantages of each scenario:

| Configuration scenario | Details                                                                                                                                                                                                                                         | Advantages | Disadvantages                                                                                                               |
| ---------------------- |-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------| ---------- |-----------------------------------------------------------------------------------------------------------------------------|
| "Full mirror" | <ul><li>Disks aren't partitioned. A mirror is made of entire disks</li><li>A single VG is used for the root system and data</li></ul>                                                                                                              | <ul><li>Reliable</li><li>Easy to configure and use</li><li>Convenient for allocating space between different software-defined storages (SDS)</li></ul> | <ul><li>Overhead disk space for SDS, which replicate data on their own</li></ul>                                            |
| "Partial mirror" | <ul><li>Disks are divided in two partitions</li><li>The first partition on each disk is used to create a mirror. It stores a VG where the operating system (OS) is installed</li><li>The second partition is used as a VG for data, without mirroring</li></ul> | <ul><li>Reliable</li><li>The most efficient disk space use</li></ul> | <ul><li>Difficult to configure and use</li><li>Very difficult to reallocate space between safe and unsafe partitions</li></ul> |

The following diagram depicts the differences in disk subsystem configuration depending on the selected scenario:

![Configuration scenarios compared](images/sds-node-configurator-scenaries.png)

## Storage with identical disks

In this scenario, you will be using a single-type disks on a node.

### Full mirror

We recommend using this configuration scenario due to its simplicity and reliability.

To configure a node according to this scenario, do the following:

1. Assemble a mirror of the entire disks (hardware or software).
   This mirror will be used for both the root system and data.
2. When installing the OS:
   - Create a VG named `main` on the mirror.
   - Create an LV named `root` in the `main` VG.
   - Install the OS on the `root` LV.
3. Add the `storage.deckhouse.io/enabled=true` tag to the `main` VG using the following command:

   ```shell
   vgchange main --addtag storage.deckhouse.io/enabled=true
   ```

4. Add the prepared node to the Deckhouse cluster.

   If the node matches the `nodeSelector` specified in `spec.nodeSelector` of the `sds-replicated-volume`
   or `sds-local-volume` modules, the `sds-node-configurator` module agent will start on that node.
   It will detect the `main` VG and add a corresponding `LVMVolumeGroup` resource to the Deckhouse cluster.
   The LVMVolumeGroup resource can then be used to create volumes in the `sds-replicated-volume` or `sds-local-volume` modules.

#### Example of SDS module configuration (identical disks, "Full mirror")

In this example, it's assumed that you have configured three nodes following the "Full mirror" scenario.
In this case, the Deckhouse cluster will have three LVMVolumeGroup resources with randomly generated names.
In the future, it will be possible to specify a name for the LVMVolumeGroup resources
created during automatic VG discovery by adding the `LVM` tag with the desired resource name.

To list the LVMVolumeGroup resources, run the following command:

```shell
kubectl get lvmvolumegroups.storage.deckhouse.io
```

In the output, you should see the following list:

```console
NAME                                      THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE      ALLOCATED SIZE   VG     AGE
vg-08d3730c-9201-428d-966c-45795cba55a6   0/0         True                    Ready   worker-2   25596Mi   0                main   61s
vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d   0/0         True                    Ready   worker-0   25596Mi   0                main   4m17s
vg-c7863e12-c143-42bb-8e33-d578ce50d6c7   0/0         True                    Ready   worker-1   25596Mi   0                main   108s
```

##### Configuring the `sds-local-volume` module (identical disks, "Full mirror")

To configure the `sds-local-volume` module following the "Full mirror" scenario,
create a LocalStorageClass resource and include all your LVMVolumeGroup resources
to use the `main` VG on all your nodes in the `sds-local-volume` module:

```yaml
kubectl apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: LocalStorageClass
metadata:
  name: local-sc
spec:
  lvm:
    lvmVolumeGroups:
      - name: vg-08d3730c-9201-428d-966c-45795cba55a6
      - name: vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d
      - name: vg-c7863e12-c143-42bb-8e33-d578ce50d6c7
    type: Thick
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
EOF
```

##### Configuring the `sds-replicated-volume` module (identical disks, "Full mirror")

To configure the `sds-replicated-volume` module according to the "Full mirror" scenario, do the following:

1. Create a ReplicatedStoragePool resource and add all your LVMVolumeGroup resources
   to use the `main` VG on all your nodes in the `sds-replicated-volume` module:

   ```yaml
   kubectl apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStoragePool
   metadata:
     name: data
   spec:
     type: LVM
     lvmVolumeGroups:
       - name: vg-08d3730c-9201-428d-966c-45795cba55a6
       - name: vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d
       - name: vg-c7863e12-c143-42bb-8e33-d578ce50d6c7
   EOF
   ```

2. Create a ReplicatedStorageClass resource
   and specify a name of the previously created ReplicatedStoragePool resource in the `storagePool` field:

   ```yaml
   kubectl apply -f -<<EOF
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-r1
   spec:
     storagePool: data
     replication: None
     reclaimPolicy: Delete
     topology: Ignored # When specifying this topology, ensure the cluster has no zones (nodes labeled with `topology.kubernetes.io/zone`).
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-r2
   spec:
     storagePool: data
     replication: Availability
     reclaimPolicy: Delete
     topology: Ignored
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-r3
   spec:
     storagePool: data
     replication: ConsistencyAndAvailability
     reclaimPolicy: Delete
     topology: Ignored
   EOF
   ```

### Partial mirror

{% alert level="warning" %}
Using partitions with the same PARTUUID is not supported, as well as changing the PARTUUID of a partition used for creating a VG. When creating a partition table, we recommended that you choose the `GPT` format, as PARTUUID in `MBR` are pseudo-random and contain the partition number. Additionally, `MBR` does not support PARTLABEL, which can be helpful to identify a partition in Deckhouse later.
{% endalert %}

In this scenario, two partitions on each disk are used:
one for the root system and SDS data storage that is not replicated,
and another for SDS data that is replicated.
The first partition of each disk is used to create a mirror,
and the second is used to create a separate VG without mirroring.
This approach maximizes the efficient use of disk space.

To configure a node according to this scenario, do the following:

1. When installing the OS:
   - Create two partitions on each disk.
   - Create a mirror from the first partitions on each disk.
   - Create a VG named `main-safe` on the mirror.
   - Create an LV named `root` in the `main-safe` VG.
   - Install the OS on the `root` LV.
2. Add the `storage.deckhouse.io/enabled=true` tag to the `main-safe` VG using the following command:

   ```shell
   vgchange main-safe --addtag storage.deckhouse.io/enabled=true
   ```

3. Create a VG named `main-unsafe` from the second partitions of each disk.
4. Add the `storage.deckhouse.io/enabled=true` tag to the `main-unsafe` VG using the following command:

   ```shell
   vgchange main-unsafe --addtag storage.deckhouse.io/enabled=true
   ```

5. Add the prepared node to the Deckhouse cluster.

   If the node matches the `nodeSelector` specified in `spec.nodeSelector` of the `sds-replicated-volume` or `sds-local-volume` modules, the `sds-node-configurator` module agent will start on that node. It will detect the `main-safe` and `main-unsafe` VG and add a corresponding LVMVolumeGroup resources to the Deckhouse cluster. These LVMVolumeGroup resources can then be used to create volumes in the `sds-replicated-volume` or `sds-local-volume` modules.

#### Example of SDS module configuration (identical disks, "Partial mirror")

In this example, it's assumed that you have configured three nodes following the "Partial mirror" scenario.
In this case, the Deckhouse cluster will have six LVMVolumeGroup resources with randomly generated names.
In the future, it will be possible to specify a name for the LVMVolumeGroup resources created during automatic VG discovery
by adding the `LVM` tag with the desired resource name.

To list the LVMVolumeGroup resources, run the following command:

```shell
kubectl get lvmvolumegroups.storage.deckhouse.io
```

In the output, you should see the following list:

```console
NAME                                      THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE      ALLOCATED SIZE   VG            AGE
vg-08d3730c-9201-428d-966c-45795cba55a6   0/0         True                    Ready   worker-2   25596Mi   0                main-safe     61s
vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d   0/0         True                    Ready   worker-0   25596Mi   0                main-safe     4m17s
vg-c7863e12-c143-42bb-8e33-d578ce50d6c7   0/0         True                    Ready   worker-1   25596Mi   0                main-safe     108s
vg-deccf08a-44d4-45f2-aea9-6232c0eeef91   0/0         True                    Ready   worker-2   25596Mi   0                main-unsafe   61s
vg-e0f00cab-03b3-49cf-a2f6-595628a2593c   0/0         True                    Ready   worker-0   25596Mi   0                main-unsafe   4m17s
vg-fe679d22-2bc7-409c-85a9-9f0ee29a6ca2   0/0         True                    Ready   worker-1   25596Mi   0                main-unsafe   108s
```

##### Configuring the `sds-local-volume` module (identical disks, "Partial mirror")

To configure the `sds-local-volume` module following the "Partial mirror" scenario,
create a LocalStorageClass resource and include the LVMVolumeGroup resources
to use only the `main-safe` VG on all your nodes in the `sds-local-volume` module:

```yaml
kubectl apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: LocalStorageClass
metadata:
  name: local-sc
spec:
  lvm:
    lvmVolumeGroups:
      - name: vg-08d3730c-9201-428d-966c-45795cba55a6
      - name: vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d
      - name: vg-c7863e12-c143-42bb-8e33-d578ce50d6c7
    type: Thick
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
EOF
```

##### Configuring the `sds-replicated-volume` module (identical disks, "Partial mirror")

To configure the `sds-replicated-volume` module according to the "Partial mirror" scenario, do the following:

1. Create a ReplicatedStoragePool resource named `data-safe` and include LVMVolumeGroup resources
   to use only the `main-safe` VG on all your nodes in the `sds-replicated-volume` module
   in ReplicatedStorageClass with the `replication: None` parameter:

   ```yaml
   kubectl apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStoragePool
   metadata:
     name: data-safe
   spec:
     type: LVM
     lvmVolumeGroups:
       - name: vg-08d3730c-9201-428d-966c-45795cba55a6
       - name: vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d
       - name: vg-c7863e12-c143-42bb-8e33-d578ce50d6c7
   EOF
   ```

2. Create a ReplicatedStoragePool resource named `data-unsafe` and include the LVMVolumeGroup resources
   to use only the `main-unsafe` VG on all your nodes in the `sds-replicated-volume` module
   in ReplicatedStorageClass with `replication: Availability` or `replication: ConsistencyAndAvailability` parameter:

   ```yaml
   kubectl apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStoragePool
   metadata:
     name: data-unsafe
   spec:
     type: LVM
     lvmVolumeGroups:
       - name: vg-deccf08a-44d4-45f2-aea9-6232c0eeef91
       - name: vg-e0f00cab-03b3-49cf-a2f6-595628a2593c
       - name: vg-fe679d22-2bc7-409c-85a9-9f0ee29a6ca2
   EOF
   ```

3. Create a ReplicatedStorageClass resource and specify a name of the previously created ReplicatedStoragePool resources
   in the `storagePool` field to use the `main-safe` and `main-unsafe` VG on all your nodes:

   ```yaml
   kubectl apply -f -<<EOF
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-r1
   spec:
     storagePool: data-safe # Note that you should use `data-safe` for this resource because it has `replication: None`, meaning there will be no replication of data for persistent volumes (PV) created with this StorageClass.
     replication: None
     reclaimPolicy: Delete
     topology: Ignored # When specifying this topology, ensure the cluster has no zones (nodes labeled with `topology.kubernetes.io/zone`).
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-r2
   spec:
     storagePool: data-unsafe # Note that you should use `data-unsafe` for this resource because it has `replication: Availability`, meaning there will be replication of data for PV created with this StorageClass.
     replication: Availability
     reclaimPolicy: Delete
     topology: Ignored
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-r3
   spec:
     storagePool: data-unsafe # Note that you should use `data-unsafe` for this resource because it has `replication: ConsistencyAndAvailability`, meaning there will be replication of data for PV created with this StorageClass.
     replication: ConsistencyAndAvailability
     reclaimPolicy: Delete
     topology: Ignored
   EOF
   ```

## Combined storage

With the combined storage, you will be using various disk types on a node.

In the case when you use various disk types to create a storage,
we recommended that you create a mirror from single-type disks and install the OS following the ["Full mirror" scenario](#full-mirror)
but don't use it for SDS.

For SDS, we recommend using disks of other types (hereinafter, additional disks),
different from the ones used for the mirror with the OS.

The following table contains recommendations on using additional disks depending on the type:

| Disk type | Recommended use |
| --------- | --------------- |
| NVMe SSD | To create volumes that require high performance. |
| SATA SSD | To create volumes that do not require high performance. |
| HDD      | To create volumes that do not require high performance. |

You can configure additional disks following either the "Full mirror" or "Partial mirror" scenario.

In the following sections, you can find configuration scenarios for additional disks of these types:

- NVMe SSD.
- SATA SSD.
- HDD.

### Configuring additional disks ("Full mirror")

{% alert level="warning" %}
The following procedure describes configuration of additional disks for initial cluster deployment and configuration
when you connect to nodes using SSH.
If you have an already running cluster and you need to connect additional disks to its nodes,
we recommend that you create and configure a VG using the [LVMVolumeGroup resource](./usage.html#creating-an-lvmvolumegroup-resource),
instead of running the commands below.
{% endalert %}

To configure additional disks on a node according to the "Full mirror" scenario, do the following:

1. Create a mirror from all additional disks of a single type (hardware or software).
2. Create a VG named `vg-name` on the mirror.
3. Assign the `storage.deckhouse.io/enabled=true` tag for `vg-name` VG using the following command:

   ```shell
   vgchange <vg-name> --addtag storage.deckhouse.io/enabled=true
   ```

{% alert level="info" %}
In the example command, replace `<vg-name>` with a corresponding VG name, depending on the type of additional disks.

Example of VG names for various disk types:

- `ssd-nvme`: For NVMe SSD.
- `ssd-sata`: For SATA SSD.
- `hdd`: For HDD.
{% endalert %}

#### Example of SDS module configuration (combined storage, "Full mirror")

In this example, it's assumed that you have configured three nodes following the "Full mirror" scenario.
In this case, the Deckhouse cluster will have three LVMVolumeGroup resources with randomly generated names.
In the future, it will be possible to specify a name for the LVMVolumeGroup resources
created during automatic VG discovery by adding the `LVM` tag with the desired resource name.

To list the LVMVolumeGroup resources, run the following command:

```shell
kubectl get lvmvolumegroups.storage.deckhouse.io
```

In the output, you should see the following list:

```console
NAME                                      THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE      ALLOCATED SIZE   VG         AGE
vg-08d3730c-9201-428d-966c-45795cba55a6   0/0         True                    Ready   worker-2   25596Mi   0                <vg-name>   61s
vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d   0/0         True                    Ready   worker-0   25596Mi   0                <vg-name>   4m17s
vg-c7863e12-c143-42bb-8e33-d578ce50d6c7   0/0         True                    Ready   worker-1   25596Mi   0                <vg-name>   108s
```

Where `<vg-name>` is the name you assigned previously.

##### Configuring the `sds-local-volume` module (combined storage, "Full mirror")

To configure the `sds-local-volume` module following the "Full mirror" scenario,
create a LocalStorageClass resource and include all LVMVolumeGroup resources
to use the `<vg-name>` VG on all nodes in the `sds-local-volume` module:

```yaml
kubectl apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: LocalStorageClass
metadata:
  name: <local-storage-class-name>
spec:
  lvm:
    lvmVolumeGroups:
      - name: vg-08d3730c-9201-428d-966c-45795cba55a6
      - name: vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d
      - name: vg-c7863e12-c143-42bb-8e33-d578ce50d6c7
    type: Thick
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
EOF
```

{% alert level="info" %}
In the example configuration, replace `<local-storage-class-name>` with a corresponding name,
depending on the type of additional disks.

Examples of the LocalStorageClass resource names for additional disks of various types:

- `local-sc-ssd-nvme`: For NVMe SSD.
- `local-sc-ssd-sata`: For SATA SSD.
- `local-sc-ssd-hdd`: For HDD.
{% endalert %}

##### Configuring the `sds-replicated-volume` module (combined storage, "Full mirror")

To configure the `sds-replicated-volume` module according to the "Full mirror" scenario, do the following:

1. Create a ReplicatedStoragePool resource and include all LVMVolumeGroup resources
   to use the `<vg-name>` VG on all nodes in the `sds-replicated-volume` module:

   ```yaml
   kubectl apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStoragePool
   metadata:
     name: <replicated-storage-pool-name>
   spec:
     type: LVM
     lvmVolumeGroups:
       - name: vg-08d3730c-9201-428d-966c-45795cba55a6
       - name: vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d
       - name: vg-c7863e12-c143-42bb-8e33-d578ce50d6c7
   EOF
   ```

   > In the example configuration, replace `<replicated-storage-pool-name>` with a corresponding name,
   > depending on the type of additional disks.
   >
   > Examples of the ReplicatedStoragePool resource names for additional disks of various types:
   >
   > - `data-ssd-nvme`: For NVMe SSD.
   > - `data-ssd-sata`: For SATA SSD.
   > - `data-hdd`: For HDD.

2. Create a ReplicatedStorageClass resource
   and specify a name of the previously created ReplicatedStoragePool resource in the `storagePool` field:

   ```yaml
   kubectl apply -f -<<EOF
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-ssd-nvme-r1
   spec:
     storagePool: <replicated-storage-pool-name>
     replication: None
     reclaimPolicy: Delete
     topology: Ignored # When specifying this topology, ensure the cluster has no zones (nodes labeled with `topology.kubernetes.io/zone`).
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-ssd-nvme-r2
   spec:
     storagePool: <replicated-storage-pool-name>
     replication: Availability
     reclaimPolicy: Delete
     topology: Ignored
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-ssd-nvme-r3
   spec:
     storagePool: <replicated-storage-pool-name>
     replication: ConsistencyAndAvailability
     reclaimPolicy: Delete
     topology: Ignored
   EOF
   ```

### Configuring additional disks ("Partial mirror")

{% alert level="warning" %}
Using partitions with the same PARTUUID is not supported, as well as changing the PARTUUID of a partition used for creating a VG. When creating a partition table, we recommended that you choose the `GPT` format, as PARTUUID in `MBR` are pseudo-random and contain the partition number. Additionally, `MBR` does not support PARTLABEL, which can be helpful to identify a partition in Deckhouse later.
{% endalert %}

{% alert level="warning" %}
The following procedure describes configuration of additional disks for initial cluster deployment and configuration
when you connect to nodes using SSH.
If you have an already running cluster and you need to connect additional disks to its nodes,
we recommend that you create and configure a VG using the [LVMVolumeGroup resource](./usage.html#creating-an-lvmvolumegroup-resource),
instead of running the commands below.
{% endalert %}

In the "Partial mirror" scenario, you will be using two partitions on each disk:
one to store non-replicable SDS data
and the other one to store replicable SDS data.
The first partition of each disk is used to create a mirror,
while the second partition is used to create a separate VG without mirroring.
This approach maximizes the efficient use of disk space.

To configure a node with additional disks according to the "Partial mirror" scenario, do the following:

1. Create two partitions on each additional disk.
2. Create a mirror from the first partitions on each disk.
3. Create a VG named `<vg-name>-safe` on the mirror.
4. Create a VG named `<vg-name>-unsafe` from the second partitions on each disk.
5. Assign the `storage.deckhouse.io/enabled=true` tag for the `ssd-nvme-safe` and `ssd-nvme-unsafe` VG using the following commands:

   ```shell
   vgchange ssd-nvme-safe --addtag storage.deckhouse.io/enabled=true
   vgchange ssd-nvme-unsafe --addtag storage.deckhouse.io/enabled=true
   ```

   > In the example commands, replace `<vg-name>` with a corresponding VG name, depending on the type of additional disks.
   >
   > Example of VG names for various disk types:
   >
   > - `ssd-nvme`: For NVMe SSD.
   > - `ssd-sata`: For SATA SSD.
   > - `hdd`: For HDD.

#### Example of SDS module configuration (combined storage, "Partial mirror")

In this example, it's assumed that you have configured three nodes following the "Partial mirror" scenario.
In this case, the Deckhouse cluster will have six LVMVolumeGroup resources with randomly generated names.
In the future, it will be possible to specify a name for the LVMVolumeGroup resources
created during automatic VG discovery by adding the `LVM` tag with the desired resource name.

To list the LVMVolumeGroup resources, run the following command:

```shell
kubectl get lvmvolumegroups.storage.deckhouse.io
```

In the output, you should see the following list:

```console
NAME                                      THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE      ALLOCATED SIZE   VG                AGE
vg-08d3730c-9201-428d-966c-45795cba55a6   0/0         True                    Ready   worker-2   25596Mi   0                <vg-name>-safe     61s
vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d   0/0         True                    Ready   worker-0   25596Mi   0                <vg-name>-safe     4m17s
vg-c7863e12-c143-42bb-8e33-d578ce50d6c7   0/0         True                    Ready   worker-1   25596Mi   0                <vg-name>-safe     108s
vg-deccf08a-44d4-45f2-aea9-6232c0eeef91   0/0         True                    Ready   worker-2   25596Mi   0                <vg-name>-unsafe   61s
vg-e0f00cab-03b3-49cf-a2f6-595628a2593c   0/0         True                    Ready   worker-0   25596Mi   0                <vg-name>-unsafe   4m17s
vg-fe679d22-2bc7-409c-85a9-9f0ee29a6ca2   0/0         True                    Ready   worker-1   25596Mi   0                <vg-name>-unsafe   108s
```

Where `<vg-name>` is the name you assigned previously.

##### Configuring the `sds-local-volume` module (combined storage, "Partial mirror")

To configure the `sds-local-volume` module following the "Partial mirror" scenario,
create a LocalStorageClass resource and include LVMVolumeGroup resources
to use only the `<vg-name>-safe` VG on all nodes in the `sds-local-volume` module:

```yaml
kubectl apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: LocalStorageClass
metadata:
  name: <local-storage-class-name>
spec:
  lvm:
    lvmVolumeGroups:
      - name: vg-08d3730c-9201-428d-966c-45795cba55a6
      - name: vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d
      - name: vg-c7863e12-c143-42bb-8e33-d578ce50d6c7
    type: Thick
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
EOF
```

{% alert level="info" %}
In the example configuration, replace `<local-storage-class-name>` with a corresponding name,
depending on the type of additional disks.

Examples of the LocalStorageClass resource names for additional disks of various types:

- `local-sc-ssd-nvme`: For NVMe SSD.
- `local-sc-ssd-sata`: For SATA SSD.
- `local-sc-ssd-hdd`: For HDD.
{% endalert %}

##### Configuring the `sds-replicated-volume` module (combined storage, "Partial mirror")

To configure the `sds-replicated-volume` module according to the "Partial mirror" scenario, do the following:

1. Create a ReplicatedStoragePool resource named `data-<vg-name>-safe` and include LVMVolumeGroup resources
   for using only the `<vg-name>-safe` VG on all nodes in the `sds-replicated-volume` module in ReplicatedStorageClass
   with the `replication: None` parameter:

   ```yaml
   kubectl apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStoragePool
   metadata:
     name: data-<vg-name>-safe
   spec:
     type: LVM
     lvmVolumeGroups:
       - name: vg-08d3730c-9201-428d-966c-45795cba55a6
       - name: vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d
       - name: vg-c7863e12-c143-42bb-8e33-d578ce50d6c7
   EOF
   ```

   > In the example configuration, replace `data-<vg-name>-safe` with a corresponding VG name,
   > depending on the type of additional disks.
   >
   > Example of the ReplicatedStoragePool resource names for additional disks of various types:
   >
   > - `data-ssd-nvme-safe`: For NVMe SSD.
   > - `data-ssd-sata-safe`: For SATA SSD.
   > - `data-hdd-safe`: For HDD.

2. Create a ReplicatedStoragePool resource named `data-<vg-name>-unsafe` and include LVMVolumeGroup resources
   for using only the `<vg-name>-unsafe` VG on all nodes in the `sds-replicated-volume` module in ReplicatedStorageClass
   with the `replication: Availability` or `replication: ConsistencyAndAvailability` parameter:

   ```yaml
   kubectl apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStoragePool
   metadata:
     name: data-<vg-name>-unsafe
   spec:
     type: LVM
     lvmVolumeGroups:
       - name: vg-deccf08a-44d4-45f2-aea9-6232c0eeef91
       - name: vg-e0f00cab-03b3-49cf-a2f6-595628a2593c
       - name: vg-fe679d22-2bc7-409c-85a9-9f0ee29a6ca2
   EOF
   ```

   > In the example configuration, replace `data-<vg-name>-unsafe` with a corresponding VG name,
   > depending on the type of additional disks.
   >
   > Example of the ReplicatedStoragePool resource names for additional disks of various types:
   >
   > - `data-ssd-nvme-unsafe`: For NVMe SSD.
   > - `data-ssd-sata-unsafe`: For SATA SSD.
   > - `data-hdd-unsafe`: For HDD.

3. Create a ReplicatedStorageClass resource and specify a name of the previously created ReplicatedStoragePool resources
   for using `<vg-name>-safe` and `<vg-name>-unsafe` VG on all nodes:

   ```yaml
   kubectl apply -f -<<EOF
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-ssd-nvme-r1
   spec:
     storagePool: data-<vg-name>-safe # Note that you should use `data-<vg-name>-safe` for this resource because it has `replication: None`, meaning there will be no replication of data for PV created with this StorageClass.
     replication: None
     reclaimPolicy: Delete
     topology: Ignored # When specifying this topology, ensure the cluster has no zones (nodes labeled with `topology.kubernetes.io/zone`).
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-ssd-nvme-r2
   spec:
     storagePool: data-<vg-name>-unsafe # Note that you should use `data-<vg-name>-unsafe` for this resource because it has `replication: Availability`, meaning there will be replication of data for PV created with this StorageClass.
     replication: Availability
     reclaimPolicy: Delete
     topology: Ignored
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-ssd-nvme-r3
   spec:
     storagePool: data-<vg-name>-unsafe # Note that you should use `data-<vg-name>-unsafe` for this resource because it has `replication: ConsistencyAndAvailability`, meaning there will be replication of data for PV created with this StorageClass.
     replication: ConsistencyAndAvailability
     reclaimPolicy: Delete
     topology: Ignored
   EOF
   ```

   > In the example configuration, replace `data-<vg-name>-unsafe` with a corresponding VG name,
   > depending on the type of additional disks.
   >
   > Example of the ReplicatedStoragePool resource names for additional disks of various types:
   >
   > - `data-ssd-nvme-unsafe`: For NVMe SSD.
   > - `data-ssd-sata-unsafe`: For SATA SSD.
   > - `data-hdd-unsafe`: For HDD.
   >
   > Replace `data-<vg-name>-safe` with a corresponding VG name, depending on the type of additional disks.
   >
   > Example of the ReplicatedStoragePool resource names for additional disks of various types:
   >
   > - `data-ssd-nvme-safe`: For NVMe SSD.
   > - `data-ssd-sata-safe`: For SATA SSD.
   > - `data-hdd-safe`: For HDD.
