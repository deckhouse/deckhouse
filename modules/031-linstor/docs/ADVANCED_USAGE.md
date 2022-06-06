---
title: "The linstor module: advanced configuration"
---

[The simplified guide](configuration.html#linstor-storage-configuration) contains steps that automatically create storage pools and StorageClasses when an LVM volume group or LVMThin pool with the tag `linstor-<name_pool>` appears on the node. Next, we consider the steps for manually creating storage pools and StorageClasses.

To proceed further, the `linstor` CLI utility is required. Use one of the following options to use the `linstor` utility:
- Install the [kubectl-linstor](https://github.com/piraeusdatastore/kubectl-linstor) plugin.
- Add a BASH alias to run  the `linstor` utility from the linstor Pod of the linstor controller:

  ```shell
  alias linstor='kubectl exec -n d8-linstor deploy/linstor-controller -- linstor'
  ```

After enabling the module, the cluster is automatically configured to use LINSTOR. In order to start using the storage, you need to:

- [Create storage pools](#creating-storage-pools)
- [Create StorageClass](#creating-a-storageclass) 

## Creating storage pools

1. Get a list of all nodes and block storage devices.
   - Get a list of all nodes in the cluster:

     ```shell
     linstor node list
     ```

     Example of the output:
  
     ```
     +----------------------------------------------------------------------------------------+
     | Node                                | NodeType   | Addresses                  | State  |
     |========================================================================================|
     | node01                              | SATELLITE  | 192.168.199.114:3367 (SSL) | Online |
     | node02                              | SATELLITE  | 192.168.199.60:3367 (SSL)  | Online |
     | node03                              | SATELLITE  | 192.168.199.74:3367 (SSL)  | Online |
     | linstor-controller-85455fcd76-2qhmq | CONTROLLER | 10.111.0.78:3367 (SSL)     | Online |
     +----------------------------------------------------------------------------------------+
     ```

   - Get a list of all available block devices for storage:

     ```shell
     linstor physical-storage list
     ```
  
     Example of the output:
  
     ```
     +----------------------------------------------------------------+
     | Size          | Rotational | Nodes                             |
     |================================================================|
     | 1920383410176 | False      | node01[/dev/nvme1n1,/dev/nvme0n1] |
     | 1920383410176 | False      | node02[/dev/nvme1n1,/dev/nvme0n1] |
     | 1920383410176 | False      | node03[/dev/nvme1n1,/dev/nvme0n1] |
     +----------------------------------------------------------------+
     ```
     
     > **Note:** you'll be able to see only empty devices without created partitions here.
     > However, creating storage pools on partitions and other block devices is also supported.
     >
     > You can also [add an existing LVM pool](faq.html#how-to-add-existing-lvm-or-lvmthin-pool) to your cluster.

1. Create an LVM or LVMThin pool of these devices.

   Create several storage pools from the devices obtained in the previous step, make them with the same name in case of using as single storageClass.

   - Example of a command to create an **LVM** storage pool of two devices on one of the nodes: 

     ```shell
     linstor physical-storage create-device-pool lvm node01 /dev/nvme0n1 /dev/nvme1n1 --pool-name linstor_data --storage-pool lvm
     ```

     , where:
     - `--pool-name` — name of the VG/LV created on the node;
     - `--storage-pool` — name of the storage pool created in LINSTOR.

   - Example of a command to create **ThinLVM** storage pool of two devices on one of the nodes:

     ```shell
     linstor physical-storage create-device-pool lvmthin node01 /dev/nvme0n1 /dev/nvme1n1 --pool-name data --storage-pool lvmthin
     ```

     , where:
     - `--pool-name` — name of the VG/LV created on the node;
     - `--storage-pool` — name of the storage pool created in LINSTOR.
     
1. Check that storage pools have been created.

   Once the storage pools are created, you can see them by executing: 
   
   ```shell
   linstor storage-pool list
   ```

   Example of the output:

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

## Creating a StorageClass

Create a StorageClass where:
- specify the required number of replicas in `parameters."linstor.csi.linbit.com/placementCount"`;  
- specify the storage pool name in `parameters."linstor.csi.linbit.com/storagePool"`.

Example of the StorageClass:

```yaml
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
