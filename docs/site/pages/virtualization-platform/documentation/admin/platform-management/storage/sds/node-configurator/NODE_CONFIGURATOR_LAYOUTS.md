---
title: "Configuration scenarios"
permalink: en/virtualization-platform/documentation/admin/platform-management/storage/sds/node-configurator/layouts.html
---

{% alert level="info" %}
Functionality is guaranteed only when using stock kernels supplied with [supported distributions](/products/virtualization-platform/documentation/about/requirements.html). When using non-standard kernels or distributions, behavior may be unpredictable.
{% endalert %}

## Cloning virtual machines

When creating virtual machines using cloning methods, replace the UUID of the volume groups (VG) by executing:

```shell
vgchange -u
```

This command will generate new UUIDs for all VGs on the virtual machine. If necessary, the command can be added to the `cloud-init` script.

{% alert level="warning" %}
Changing the UUID is only possible if there are no active logical volumes (LV) in the group. They can be deactivated as follows:

```shell
lvchange -an <VG_or_LV_NAME>
```

Where `<VG_or_LV_NAME>` is the name of the VG to deactivate all volumes in the group, or the name of the LV to deactivate a specific volume.
{% endalert %}

## Methods and scenarios for configuring the disk subsystem of nodes

The disk subsystem of each node can be organized in two ways, depending on whether the disks installed in the server are identical.

- [Storage with identical disks](#storage-with-identical-disks): All disks in the node are of the same type and size.
- [Combined storage](#combined-storage): The node has disks of different types (for example, SSD + HDD).

For each method of configuring the disk subsystem on nodes, there are two configuration scenarios:

- **Full mirror**: Recommended, reliable, and the simplest.
- **Partial mirror**: More flexible but requires caution.

The features, advantages, and disadvantages of the scenarios are presented in the table:

<table>
  <thead>
    <tr>
      <th>Configuration scenario</th>
      <th>Implementation features</th>
      <th>Advantages</th>
      <th>Disadvantages</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>Full mirror</td>
      <td>
        <ul>
          <li>Disks are not partitioned; a mirror is created from entire disks</li>
          <li>Single VG for OS and data</li>
        </ul>
      </td>
      <td>
        <ul>
          <li>High reliability</li>
          <li>Ease of setup and operation</li>
          <li>Flexible resource allocation between SDS</li>
        </ul>
      </td>
      <td>
        <ul>
          <li>Excessive disk consumption with SDS replication</li>
        </ul>
      </td>
    </tr>
    <tr>
      <td>Partial mirror</td>
      <td>
        <ul>
          <li>Disks are divided into 2 partitions</li>
          <li>A mirror is created from the first partitions of each disk, on which a VG for the OS is created</li>
          <li>A VG for data is created from the second partitions without mirroring</li>
        </ul>
      </td>
      <td>
        <ul>
          <li>Reliable storage</li>
          <li>Maximum efficiency in space utilization</li>
        </ul>
      </td>
      <td>
        <ul>
          <li>Complex setup</li>
          <li>Difficulties in reallocating space between safe and unsafe partitions</li>
        </ul>
      </td>
    </tr>
  </tbody>
</table>

The differences in the configuration order of the disk subsystem depending on the chosen configuration scenario are illustrated in the diagram:

![Configuration scenarios](/images/storage/sds/node-configurator/sds-node-configurator-scenaries.png)

## Storage with identical disks

### Full mirror

We recommend using this configuration scenario as it is sufficiently reliable and easy to set up.

To configure a node according to this scenario, follow these steps:

1. Create a mirror from all disks (hardware or software). This mirror will be used simultaneously for the root system and for data.
1. Install the operating system:
   - Create a VG named `main` on the mirror.
   - Create an LV named `root` in the VG `main`.
   - Install the operating system on the LV `root`.
1. Set the tag `storage.deckhouse.io/enabled=true` for the VG `main` using the following command:

   ```shell
   vgchange main --addtag storage.deckhouse.io/enabled=true
   ```

1. Add the prepared node to the DVP cluster.

   If the node matches the `nodeSelector` specified in `spec.nodeSelector` of the `sds-replicated-volume` or `sds-local-volume` modules, the `sds-node-configurator` agent will detect the VG `main` and create a [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resource. This can be used in the `sds-local-volume` and `sds-replicated-volume` modules.

#### Example of configuring SDS modules (identical disks, "Full mirror")

In this scenario, three nodes of the DVP cluster are configured in "Full mirror" mode.  
After automatic discovery, three CRD resources of type [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) with automatically generated names will appear in the cluster.

To list the [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources, execute the following command:

```shell
d8 k get lvmvolumegroups.storage.deckhouse.io
```

The result will be the following list:

```console
NAME                                      THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE      ALLOCATED SIZE   VG     AGE
vg-08d3730c-9201-428d-966c-45795cba55a6   0/0         True                    Ready   worker-2   25596Mi   0                main   61s
vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d   0/0         True                    Ready   worker-0   25596Mi   0                main   4m17s
vg-c7863e12-c143-42bb-8e33-d578ce50d6c7   0/0         True                    Ready   worker-1   25596Mi   0                main   108s
```

##### Configuring the sds-local-volume module (identical disks, "Full mirror")

To configure `sds-local-volume` in "Full Mirror" mode, create a [LocalStorageClass](/modules/sds-local-volume/stable/cr.html#localstorageclass) resource and specify all discovered [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources in it. This ensures that the VG with the label `main` is available on each node in the module:

```shell
d8 k apply -f -<<EOF
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

##### Configuring the sds-replicated-volume module (identical disks, "Full mirror")

To configure the `sds-replicated-volume` module according to the "Full mirror" scenario, follow these steps:

1. Create a [ReplicatedStoragePool](/modules/sds-replicated-volume/stable/cr.html#replicatedstoragepool) resource and add all [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources to it, so that the VG `main` is used on all nodes in the `sds-replicated-volume` module:

   ```shell
   d8 k apply -f -<<EOF
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

1. Create a [ReplicatedStorageClass](/modules/sds-replicated-volume/stable/cr.html#replicatedstorageclass) resource and specify the name of the previously created [ReplicatedStoragePool](/modules/sds-replicated-volume/stable/cr.html#replicatedstoragepool) resource in the `storagePool` field:

   ```shell
   d8 k apply -f -<<EOF
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-r1
   spec:
     storagePool: data
     replication: None
     reclaimPolicy: Delete
     topology: Ignored # If this topology is specified, there should be no zones in the cluster (nodes with labels topology.kubernetes.io/zone).
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

- Do not use partitions with the same `PARTUUID`.
- Changing the `PARTUUID` of a partition on which a VG is already created is not supported.
- It is recommended to use GPT for the partition table — in MBR, `PARTUUID` is pseudo-random and contains the partition number, and there is no support for `PARTLABEL`, which may be useful for identifying partitions in DVP.
{% endalert %}

In this scenario, two partitions are used on each disk:

- A partition for the root system and storage of SDS data that is not replicated.
- A partition for SDS data that is replicated.

The first partition of each disk is used to create a mirror, and the second is used to create a separate VG without mirroring. This allows for the most efficient use of disk space.

To configure a node according to the "Partial mirror" scenario, follow these steps:

1. During the operating system installation:
   - Create two partitions on each disk.
   - Assemble a mirror from the first partitions on each disk.
   - Create a VG named `main-safe` on the mirror.
   - Create an LV named `root` in the VG `main-safe`.
   - Install the operating system on the LV `root`.
1. Set the tag `storage.deckhouse.io/enabled=true` for the VG `main-safe` using the following command:

   ```shell
   vgchange main-safe --addtag storage.deckhouse.io/enabled=true
   ```

1. Create a VG named `main-unsafe` from the second partitions of each disk.
1. Set the tag `storage.deckhouse.io/enabled=true` for the VG `main-unsafe` using the following command:

   ```shell
   vgchange main-unsafe --addtag storage.deckhouse.io/enabled=true
   ```

1. Add the prepared node to the DVP cluster.

   If the node matches the `nodeSelector` specified in `spec.nodeSelector` of the `sds-replicated-volume` or `sds-local-volume` modules, the `sds-node-configurator` agent will run on this node, detect the VGs `main-safe` and `main-unsafe`, and add the corresponding [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources to the DVP cluster. These [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources can then be used to create volumes in the `sds-replicated-volume` or `sds-local-volume` modules.

#### Example of configuring SDS modules (identical disks, "Partial mirror")

In this example, it is assumed that you have configured three nodes according to the "Partial mirror" scenario. In the DVP cluster, six [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources with randomly generated names will appear. In the future, it will be possible to specify a name for the [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources created during the automatic VG discovery process using the `LVM` tag with the desired resource name.

To list the [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources, execute the following command:

```shell
d8 k get lvmvolumegroups.storage.deckhouse.io
```

The result will be the following list:

```console
NAME                                      THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE      ALLOCATED SIZE   VG            AGE
vg-08d3730c-9201-428d-966c-45795cba55a6   0/0         True                    Ready   worker-2   25596Mi   0                main-safe     61s
vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d   0/0         True                    Ready   worker-0   25596Mi   0                main-safe     4m17s
vg-c7863e12-c143-42bb-8e33-d578ce50d6c7   0/0         True                    Ready   worker-1   25596Mi   0                main-safe     108s
vg-deccf08a-44d4-45f2-aea9-6232c0eeef91   0/0         True                    Ready   worker-2   25596Mi   0                main-unsafe   61s
vg-e0f00cab-03b3-49cf-a2f6-595628a2593c   0/0         True                    Ready   worker-0   25596Mi   0                main-unsafe   4m17s
vg-fe679d22-2bc7-409c-85a9-9f0ee29a6ca2   0/0         True                    Ready   worker-1   25596Mi   0                main-unsafe   108s
```

##### Configuring the sds-local-volume module (identical disks, "Partial mirror")

To configure the `sds-local-volume` module according to the "Partial mirror" scenario, create a [LocalStorageClass](/modules/sds-local-volume/stable/cr.html#localstorageclass) resource and add the [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources to it so that only the VG `main-safe` is used on all nodes in the `sds-local-volume` module:

```shell
d8 k apply -f -<<EOF
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

##### Configuring the sds-replicated-volume module (identical disks, "Partial mirror")

To configure the `sds-replicated-volume` module according to the "Partial mirror" scenario, follow these steps:

1. Create a [ReplicatedStoragePool](/modules/sds-replicated-volume/stable/cr.html#replicatedstoragepool) resource named `data-safe` and add the [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources to it so that only the VG `main-safe` is used on all nodes in the `sds-replicated-volume` module for [ReplicatedStorageClass](/modules/sds-replicated-volume/stable/cr.html#replicatedstorageclass) with the parameter `replication: None`:

   ```shell
   d8 k apply -f -<<EOF
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

1. Create a [ReplicatedStoragePool](/modules/sds-replicated-volume/stable/cr.html#replicatedstoragepool) resource named `data-unsafe` and add the [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources to it so that only the VG `main-unsafe` is used on all nodes in the `sds-replicated-volume` module for [ReplicatedStorageClass](/modules/sds-replicated-volume/stable/cr.html#replicatedstorageclass) with the parameter `replication: Availability` or `replication: ConsistencyAndAvailability`:

   ```shell
   d8 k apply -f -<<EOF
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

1. Create a [ReplicatedStorageClass](/modules/sds-replicated-volume/stable/cr.html#replicatedstorageclass) resource and specify the name of the previously created [ReplicatedStoragePool](/modules/sds-replicated-volume/stable/cr.html#replicatedstoragepool) resources in the `storagePool` field so that the VGs `main-safe` and `main-unsafe` are used on all nodes:

   ```shell
   d8 k apply -f -<<EOF
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-r1
   spec:
     storagePool: data-safe # Note that due to replication: None, this resource uses data-safe; therefore, data replication for persistent volumes (PV) created with this StorageClass will not be performed.
     replication: None
     reclaimPolicy: Delete
     topology: Ignored # If this topology is specified, there should be no zones in the cluster (nodes with labels topology.kubernetes.io/zone).
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-r2
   spec:
     storagePool: data-unsafe # Note that due to replication: Availability, this resource uses data-unsafe; therefore, data replication for PVs created with this StorageClass will be performed.
     replication: Availability
     reclaimPolicy: Delete
     topology: Ignored
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-r3
   spec:
     storagePool: data-unsafe # Note that due to replication: ConsistencyAndAvailability, this resource uses data-unsafe; therefore, data replication for PVs created with this StorageClass will be performed.
     replication: ConsistencyAndAvailability
     reclaimPolicy: Delete
     topology: Ignored
   EOF
   ```

## Combined storage

Combined storage involves the simultaneous use of disks of different types on a node.

When combining disks of different types to create storage, we recommend creating a mirror from disks of one type and installing the operating system on it according to the ["Full mirror"](#full-mirror) scenario, but not using it for SDS.

For SDS, use disks of other types (hereinafter referred to as additional disks) that differ from those used for the mirror under the operating system.

Recommendations for using additional disks depending on their type:

| Disk type | Recommended use cases                                   |
|-----------|---------------------------------------------------------|
| NVMe SSD  | Creating volumes that require high performance          |
| SATA SSD  | Creating volumes that do not require high performance   |
| HDD       | Creating volumes that do not require high performance   |

Additional disks can be configured according to either the "Full mirror" or "Partial mirror" scenarios.

Below, the process of configuring additional disks will be considered using the following types as examples:

- NVMe SSD
- SATA SSD
- HDD

### Configuring additional disks (Full mirror)

{% alert level="warning" %}
The following describes the procedure for configuring additional disks for the case of initial deployment and configuration of the cluster when connecting to nodes via SSH. If you already have a running cluster and are adding additional disks to its nodes, it is recommended to create and configure VGs using the [LVMVolumeGroup](./usage.html#creating-an-lvmvolumegroup-resource) resource instead of executing the commands below on the node.
{% endalert %}

To configure additional disks on a node according to the "Full mirror" scenario, follow these steps:

1. Assemble a mirror from all additional disks of a certain type entirely (hardware or software).
1. Create a VG named `<vg-name>` on the mirror.
1. Set the tag `storage.deckhouse.io/enabled=true` for the VG `<vg-name>` using the following command:

   ```shell
   vgchange <vg-name> --addtag storage.deckhouse.io/enabled=true
   ```

{% alert level="info" %}
In the example above, replace `<vg-name>` with an informative name depending on the type of additional disks.

Examples of VG names for additional disks of different types:

- `ssd-nvme`: For NVMe SSD disks.
- `ssd-sata`: For SATA SSD disks.
- `hdd`: For HDD disks.
{% endalert %}

#### Example of configuring SDS modules (combined storage, "Full mirror")

In this example, it is assumed that you have configured three nodes according to the "Full Mirror" scenario. In the DVP cluster, three [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources with randomly generated names will appear. In the future, it will be possible to specify a name for the [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources created during the automatic VG discovery process using the `LVM` tag with the desired resource name.

To list the [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources, execute the following command:

```shell
d8 k get lvmvolumegroups.storage.deckhouse.io
```

The result will be a list like this:

```console
NAME                                      THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE      ALLOCATED SIZE   VG          AGE
vg-08d3730c-9201-428d-966c-45795cba55a6   0/0         True                    Ready   worker-2   25596Mi   0                <vg-name>   61s
vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d   0/0         True                    Ready   worker-0   25596Mi   0                <vg-name>   4m17s
vg-c7863e12-c143-42bb-8e33-d578ce50d6c7   0/0         True                    Ready   worker-1   25596Mi   0                <vg-name>   108s
```

Where `<vg-name>` is the name assigned to the VG on the mirror in the previous step.

##### Configuring the sds-local-volume module (combined storage, "Full mirror")

To configure the `sds-local-volume` module according to the "Full Mirror" scenario, create a [LocalStorageClass](/modules/sds-local-volume/stable/cr.html#localstorageclass) resource and add all [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources to it so that the VG `<vg-name>` is used on all nodes in the `sds-local-volume` module:

```shell
d8 k apply -f -<<EOF
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
In the example above, replace `<local-storage-class-name>` with an informative name depending on the type of additional disks.

Examples of informative names for the [LocalStorageClass](/modules/sds-local-volume/stable/cr.html#localstorageclass) resource for additional disks of different types:

- `local-sc-ssd-nvme`: For NVMe SSD disks.
- `local-sc-ssd-sata`: For SATA SSD disks.
- `local-sc-hdd`: For HDD disks.
{% endalert %}

##### Configuring the sds-replicated-volume module (combined storage, "Full mirror")

To configure the `sds-replicated-volume` module according to the "Full Mirror" scenario, follow these steps:

1. Create a [ReplicatedStoragePool](/modules/sds-replicated-volume/stable/cr.html#replicatedstoragepool) resource and add all [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources to it so that the VG `<vg-name>` is used on all nodes in the `sds-replicated-volume` module:

   ```shell
   d8 k apply -f -<<EOF
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

   > In the example above, replace `<replicated-storage-pool-name>` with an informative name depending on the type of additional disks.
   >
   > Examples of informative names for the [ReplicatedStoragePool](/modules/sds-replicated-volume/stable/cr.html#replicatedstoragepool) resource for additional disks of different types:
   >
   > - `data-ssd-nvme`: For NVMe SSD disks.
   > - `data-ssd-sata`: For SATA SSD disks.
   > - `data-hdd`: For HDD disks.

1. Create a [ReplicatedStorageClass](/modules/sds-replicated-volume/stable/cr.html#replicatedstorageclass) resource and specify the name of the previously created [ReplicatedStoragePool](/modules/sds-replicated-volume/stable/cr.html#replicatedstoragepool) resource in the `storagePool` field:

   ```shell
   d8 k apply -f -<<EOF
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-ssd-nvme-r1
   spec:
     storagePool: <replicated-storage-pool-name>
     replication: None
     reclaimPolicy: Delete
     topology: Ignored # If this topology is specified, there should be no zones in the cluster (nodes with labels topology.kubernetes.io/zone).
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

### Configuring additional disks (Partial mirror)

{% alert level="warning" %}

- Do not use partitions with the same `PARTUUID`.
- Changing the `PARTUUID` of a partition on which a VG is already created is not supported.
- It is recommended to use GPT for the partition table — in MBR, `PARTUUID` is pseudo-random and contains the partition number, and there is no support for `PARTLABEL`, which may be useful for identifying partitions in DVP.
{% endalert %}

{% alert level="warning" %}
The following describes the procedure for configuring additional disks for the case of initial deployment and configuration of the cluster when connecting to nodes via SSH. If you already have a running cluster and are adding additional disks to its nodes, it is recommended to create and configure VGs using the [LVMVolumeGroup](./usage.html#creating-an-lvmvolumegroup-resource) resource instead of executing the commands below on the node.
{% endalert %}

In this scenario, two partitions are used on each disk: one for storing SDS data that is not replicated and another for SDS data that is replicated. The first partition of each disk is used to create a mirror, and the second is used to create a separate VG without mirroring. This allows for the most efficient use of disk space.

To configure a node with additional disks according to the "Partial mirror" scenario, follow these steps:

1. Create two partitions on each additional disk.
1. Assemble a mirror from the first partitions on each disk.
1. Create a VG named `<vg-name>-safe` on the mirror.
1. Create a VG named `<vg-name>-unsafe` from the second partitions of each disk.
1. Set the tag `storage.deckhouse.io/enabled=true` for the VGs `<vg-name>-safe` and `<vg-name>-unsafe` using the following commands:

   ```shell
   vgchange <vg-name>-safe --addtag storage.deckhouse.io/enabled=true
   vgchange <vg-name>-unsafe --addtag storage.deckhouse.io/enabled=true
   ```

   > In the example above, replace `<vg-name>` with an informative prefix depending on the type of additional disks.
   >
   > Examples of informative prefixes `<vg-name>` for additional disks of different types:
   >
   > - `ssd-nvme`: For NVMe SSD disks.
   > - `ssd-sata`: For SATA SSD disks.
   > - `hdd`: For HDD disks.

#### Example of configuring SDS modules (Combined storage, "Partial mirror")

In this example, it is assumed that you have configured three nodes according to the "Partial Mirror" scenario. In the DVP cluster, six [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources with randomly generated names will appear. In the future, it will be possible to specify a name for the [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources created during the automatic VG discovery process using the `LVM` tag with the desired resource name.

To list the [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources, execute the following command:

```shell
d8 k get lvmvolumegroups.storage.deckhouse.io
```

The result will be a list like this:

```console
NAME                                      THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE      ALLOCATED SIZE   VG                AGE
vg-08d3730c-9201-428d-966c-45795cba55a6   0/0         True                    Ready   worker-2   25596Mi   0                <vg-name>-safe     61s
vg-b59ff9e1-6ef2-4761-b5d2-6172926d4f4d   0/0         True                    Ready   worker-0   25596Mi   0                <vg-name>-safe     4m17s
vg-c7863e12-c143-42bb-8e33-d578ce50d6c7   0/0         True                    Ready   worker-1   25596Mi   0                <vg-name>-safe     108s
vg-deccf08a-44d4-45f2-aea9-6232c0eeef91   0/0         True                    Ready   worker-2   25596Mi   0                <vg-name>-unsafe   61s
vg-e0f00cab-03b3-49cf-a2f6-595628a2593c   0/0         True                    Ready   worker-0   25596Mi   0                <vg-name>-unsafe   4m17s
vg-fe679d22-2bc7-409c-85a9-9f0ee29a6ca2   0/0         True                    Ready   worker-1   25596Mi   0                <vg-name>-unsafe   108s
```

Where `<vg-name>` is the prefix of the name assigned to the VGs created in the previous step.

##### Configuring the sds-local-volume module (combined storage, "Partial mirror")

To configure the `sds-local-volume` module according to the "Partial Mirror" scenario, create a [LocalStorageClass](/modules/sds-local-volume/stable/cr.html#localstorageclass) resource and add the [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources to it so that only the VG `<vg-name>-safe` is used on all nodes in the `sds-local-volume` module:

```shell
d8 k apply -f -<<EOF
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
In the example above, replace `<local-storage-class-name>` with an informative name depending on the type of additional disks.

Examples of informative names for the [LocalStorageClass](/modules/sds-local-volume/stable/cr.html#localstorageclass) resource for additional disks of different types:

- `local-sc-ssd-nvme`: For NVMe SSD disks.
- `local-sc-ssd-sata`: For SATA SSD disks.
- `local-sc-hdd`: For HDD disks.
{% endalert %}

##### Configuring the sds-replicated-volume module (combined storage, "Partial mirror")

To configure the `sds-replicated-volume` module according to the "Partial mirror" scenario, follow these steps:

1. Create a [ReplicatedStoragePool](/modules/sds-replicated-volume/stable/cr.html#replicatedstoragepool) resource named `data-<vg-name>-safe` and add the [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources to it so that only the VG `<vg-name>-safe` is used on all nodes in the `sds-replicated-volume` module for [ReplicatedStorageClass](/modules/sds-replicated-volume/stable/cr.html#replicatedstorageclass) with the parameter `replication: None`:

   ```shell
   d8 k apply -f -<<EOF
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

   > In the example above, replace `data-<vg-name>-safe` with an informative name depending on the type of additional disks.
   >
   > Examples of informative names for the [ReplicatedStoragePool](/modules/sds-replicated-volume/stable/cr.html#replicatedstoragepool) resource for additional disks of different types:
   >
   > - `data-ssd-nvme-safe`: For NVMe SSD disks.
   > - `data-ssd-sata-safe`: For SATA SSD disks.
   > - `data-hdd-safe`: For HDD disks.

1. Create a [ReplicatedStoragePool](/modules/sds-replicated-volume/stable/cr.html#replicatedstoragepool) resource named `data-<vg-name>-unsafe` and add the [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resources to it so that only the VG `<vg-name>-unsafe` is used on all nodes in the `sds-replicated-volume` module for [ReplicatedStorageClass](/modules/sds-replicated-volume/stable/cr.html#replicatedstorageclass) with the parameter `replication: Availability` or `replication: ConsistencyAndAvailability`:

   ```shell
   d8 k apply -f -<<EOF
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

   > In the example above, replace `data-<vg-name>-unsafe` with an informative name depending on the type of additional disks.
   >
   > Examples of informative names for the [ReplicatedStoragePool](/modules/sds-replicated-volume/stable/cr.html#replicatedstoragepool) resource for additional disks of different types:
   >
   > - `data-ssd-nvme-unsafe`: For NVMe SSD disks.
   > - `data-ssd-sata-unsafe`: For SATA SSD disks.
   > - `data-hdd-unsafe`: For HDD disks.

1. Create a [ReplicatedStorageClass](/modules/sds-replicated-volume/stable/cr.html#replicatedstorageclass) resource and specify the name of the previously created [ReplicatedStoragePool](/modules/sds-replicated-volume/stable/cr.html#replicatedstoragepool) resources in the `storagePool` field so that the VGs `<vg-name>-safe` and `<vg-name>-unsafe` are used on all nodes:

   ```shell
   d8 k apply -f -<<EOF
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-ssd-nvme-r1
   spec:
     storagePool: data-<vg-name>-safe # Note that due to replication: None, this resource uses data-<vg-name>-safe; therefore, data replication for PVs created with this StorageClass will not be performed.
     replication: None
     reclaimPolicy: Delete
     topology: Ignored # If this topology is specified, there should be no zones in the cluster (nodes with labels topology.kubernetes.io/zone).
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-ssd-nvme-r2
   spec:
     storagePool: data-<vg-name>-unsafe # Note that due to replication: Availability, this resource uses data-<vg-name>-unsafe; therefore, data replication for PVs created with this StorageClass will be performed.
     replication: Availability
     reclaimPolicy: Delete
     topology: Ignored
   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: replicated-sc-ssd-nvme-r3
   spec:
     storagePool: data-<vg-name>-unsafe # Note that due to replication: ConsistencyAndAvailability, this resource uses data-<vg-name>-unsafe; therefore, data replication for PVs created with this StorageClass will be performed.
     replication: ConsistencyAndAvailability
     reclaimPolicy: Delete
     topology: Ignored
   EOF
   ```

   > In the example above, replace `data-<vg-name>-unsafe` with an informative name depending on the type of additional disks.
   >
   > Examples of informative names for the [ReplicatedStoragePool](/modules/sds-replicated-volume/stable/cr.html#replicatedstoragepool) resource for additional disks of different types:
   >
   > - `data-ssd-nvme-unsafe`: For NVMe SSD disks.
   > - `data-ssd-sata-unsafe`: For SATA SSD disks.
   > - `data-hdd-unsafe`: For HDD disks.
   >
   > In a similar way, replace `data-<vg-name>-safe`.
   >
   > Examples of informative names for the [ReplicatedStoragePool](/modules/sds-replicated-volume/stable/cr.html#replicatedstoragepool) resource for additional disks of different types:
   >
   > - `data-ssd-nvme-safe`: For NVMe SSD disks.
   > - `data-ssd-sata-safe`: For SATA SSD disks.
   > - `data-hdd-safe`: For HDD disks.
