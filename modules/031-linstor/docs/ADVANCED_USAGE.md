---
title: "The linstor module: advanced configuration examples"
---

To continue, it is recommended to install the [kubectl-linstor](https://github.com/piraeusdatastore/kubectl-linstor) plugin or add a bash-alias:

```shell
alias linstor='kubectl exec -n d8-linstor deploy/linstor-controller -- linstor'
```

Further configuration is performed using the `linstor` command utility.

Nodes are already set up automatically. In order to start using LINSTOR, you need to do two things: 

- Create storage pools;
- Describe the desired options in the StorageClass. 

## Create storage pools

To list all nodes in the cluster, run:
```shell
linstor node list
```

To list all available block devices for storage, run:
```shell
linstor physical-storage list
```

Example output:

```
+----------------------------------------------------------------+
| Size          | Rotational | Nodes                             |
|================================================================|
| 1920383410176 | False      | node01[/dev/nvme1n1,/dev/nvme0n1] |
| 1920383410176 | False      | node02[/dev/nvme1n1,/dev/nvme0n1] |
| 1920383410176 | False      | node03[/dev/nvme1n1,/dev/nvme0n1] |
+----------------------------------------------------------------+
```

> Warning: You'll be able to see only empty devices without created partitions here.
> However, creating storage pools on partitions and other block devices is also supported.

> You can also add an existing LVM pool to your cluster, see [FAQ](faq.html#how-to-add-existing-lvm-or-lvmthin-pool) for this.

Create an LVM or LVMThin pool of these devices:

- To create an LVM storage pool of two devices on one of the nodes, run the following command: 

  ```shell
  linstor physical-storage create-device-pool lvm node01 /dev/nvme0n1 /dev/nvme1n1 --pool-name linstor_data --storage-pool lvm
  ```
  
- To create an LVMThin storage pool of two devices on one of the nodes, run the following command: 

  ```shell
  linstor physical-storage create-device-pool lvmthin node01 /dev/nvme0n1 /dev/nvme1n1 --pool-name data --storage-pool lvmthin
  ```

The options are:
- `--pool-name` — name of the VG/LV created on the node;
- `--storage-pool` — name of the storage-pool created in LINSTOR. 

You need to create several such pools for each of your nodes, make them with the same name if possible.

Once the pools are created, you can see them by executing: 

```shell
linstor storage-pool list
```

Example output:

```
+---------------------------------------------------------------------------------------------------------------------------------+
| StoragePool          | Node   | Driver   | PoolName          | FreeCapacity | TotalCapacity | CanSnapshots | State | SharedName |
|=================================================================================================================================|
| DfltDisklessStorPool | node01 | DISKLESS |                   |              |               | False        | Ok    |            |
| DfltDisklessStorPool | node02 | DISKLESS |                   |              |               | False        | Ok    |            |
| DfltDisklessStorPool | node03 | DISKLESS |                   |              |               | False        | Ok    |            |
| lvmthin              | node01 | LVM_THIN | linstor_data/data |     3.49 TiB |      3.49 TiB | True         | Ok    |            |
| lvmthin              | node02 | LVM_THIN | linstor_data/data |     3.49 TiB |      3.49 TiB | True         | Ok    |            |
| lvmthin              | node03 | LVM_THIN | linstor_data/data |     3.49 TiB |      3.49 TiB | True         | Ok    |            |
+---------------------------------------------------------------------------------------------------------------------------------+
```

## Create StorageClass

Now specify the desired number of replicas and the storage-pool name for them in your StorageClass and apply it to Kubernetes:

```
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: linstor-r2
parameters:
  linstor.csi.linbit.com/storagePool: lvmthin
  linstor.csi.linbit.com/placementCount: "2"
allowVolumeExpansion: true
provisioner: linstor.csi.linbit.com
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer
```

Now the configuration can be considered complete.
