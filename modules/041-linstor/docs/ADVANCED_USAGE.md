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

> Most of the items on this page are taken from the [official LINSTOR documentation](https://linbit.com/drbd-user-guide/linstor-guide-1_0-en/).
> Despite the fact that here we have tried to collect the most common questions, feel free to refer to the original source.

## Manual configuration

After enabling the module, the cluster is automatically configured to use LINSTOR. In order to start using the storage, you need to:

- [Create storage pools](#creating-storage-pools)
- [Create StorageClass](#creating-a-storageclass)

### Creating storage pools

1. Get a list of all nodes and block storage devices.
   - Get a list of all nodes in the cluster:

     ```shell
     linstor node list
     ```

     Example of the output:
  
     ```text
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
  
     ```text
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

   ```text
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

### Creating a StorageClass

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
  property.linstor.csi.linbit.com/DrbdOptions/Net/rr-conflict: retry-connect
  property.linstor.csi.linbit.com/DrbdOptions/Resource/on-no-data-accessible: suspend-io
  property.linstor.csi.linbit.com/DrbdOptions/Resource/on-suspended-primary-outdated: force-secondary
  property.linstor.csi.linbit.com/DrbdOptions/auto-quorum: suspend-io
allowVolumeExpansion: true
provisioner: linstor.csi.linbit.com
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer
```

## Backup on S3 Storage

> This feature requires a configured master passphrase (see instructions on the top of the [module configuration](configuration.html) page).
>
> Snapshots are supported only for LVMThin pools.

Data backup is implemented using [volume snapshots](https://kubernetes.io/docs/concepts/storage/volume-snapshots/). Snapshots are supported by the [snapshot-controller](../045-snapshot-controller/) module, which is enabled automatically for supported CSI drivers in Kubernetes clusters versions 1.20 and higher.

### Creating a backup

To create a snapshot of a volume and upload it to S3, follow these steps:
1. Create `VolumeSnapshotClass` and `Secret`, containing the access key and secret key of S3 storage.

   > VolumeSnapshotClass is a cluster-wide resource. The same VolumeSnapshotClass can be used to create backups of different PVCs from different namespaces.

   Example of `VolumeSnapshotClass` and `Secret`:

   ```yaml
   kind: VolumeSnapshotClass
   apiVersion: snapshot.storage.k8s.io/v1
   metadata:
     name: linstor-csi-snapshot-class-s3
   driver: linstor.csi.linbit.com
   deletionPolicy: Retain
   parameters:
     snap.linstor.csi.linbit.com/type: S3
     snap.linstor.csi.linbit.com/remote-name: backup-remote               # Linstor remote name.   
     snap.linstor.csi.linbit.com/allow-incremental: "false"               # Whether to use incremental copies. 
     snap.linstor.csi.linbit.com/s3-bucket: snapshot-bucket               # The name of the S3 bucket, for data storage.
     snap.linstor.csi.linbit.com/s3-endpoint: s3.us-west-1.amazonaws.com  # S3 endpoint URL.
     snap.linstor.csi.linbit.com/s3-signing-region: us-west-1             # The name of the S3 region. 
     # Use virtual hosted–style or path-style S3 URL. 
     # https://docs.aws.amazon.com/AmazonS3/latest/userguide/VirtualHosting.html
     snap.linstor.csi.linbit.com/s3-use-path-style: "false"    
     # Secret, containing the access key and secret key of access to S3 bucket.
     csi.storage.k8s.io/snapshotter-secret-name: linstor-csi-s3-access
     csi.storage.k8s.io/snapshotter-secret-namespace: storage
   ---
   kind: Secret
   apiVersion: v1
   metadata:
     name: linstor-csi-s3-access
     namespace: storage
   immutable: true
   type: linstor.csi.linbit.com/s3-credentials.v1
   stringData:
     access-key: *!ACCESS_KEY*  # S3 access key.
     secret-key: *!SECRET_KEY*  # S3 secret key.
   ```

1. Select (or create) `PersistentVolumeClaim`, whose data needs to be copied.

   Example of `PersistentVolumeClaim`, which will be used in the examples below:

   ```yaml
   apiVersion: v1
   kind: PersistentVolumeClaim
   metadata:
     name: my-linstor-volume
     namespace: storage
   spec:
     accessModes:
     - ReadWriteOnce
     storageClassName: linstor-thindata-r2   # The name of the StorageClass.
     resources:
       requests:
         storage: 2Gi
   ```

1. Create `VolumeSnapshot`.

   Example of `VolumeSnapshot`, using `VolumeSnapshotClass` created earlier:

   ```yaml
   apiVersion: snapshot.storage.k8s.io/v1
   kind: VolumeSnapshot
   metadata:
     name: my-linstor-snapshot
     namespace: storage
   spec:
     volumeSnapshotClassName: linstor-csi-snapshot-class-s3   # The name of the VolumeSnapshotClass, with access to the S3 storage.
     source:
       persistentVolumeClaimName: my-linstor-volume           # The name of the PVC, from where to copy the data. 
   ```

   After creating a `VolumeSnapshot` associated with a `PersistentVolumeClaim` related to an existing volume, a linstor snapshot will be created and uploaded to S3 storage.

1. Check the backup status.

   Example:

   ```shell
   kubectl get volumesnapshot my-linstor-snapshot -n storage
   ```

   If the READYTOUSE value of `VolumeSnapshot` is not `true`, then see the reason by running the following command:  

   ```shell
   kubectl describe volumesnapshot my-linstor-snapshot -n storage
   ```

To view the list and status of created snapshots in linstor run the following command:

```shell
linstor snapshot list
```

### Restoring from a backup

It is enough to create a PVC referencing the required VolumeSnapshot to restore data in the same namespace in which VolumeSnapshot was created.

An example of a PVC for restoring from VolumeSnapshot `example-backup-from-s3` in the same namespace:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: restored-data
  namespace: storage
spec:
  storageClassName: "linstor-thindata-r1" # The name of the StorageClass.  
  dataSource:
    name: example-backup-from-s3          # The name of the VolumeSnapshot, created earlier.
    kind: VolumeSnapshot
    apiGroup: snapshot.storage.k8s.io
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 2Gi
```

To recover data from S3 storage into another namespace or Kubernetes cluster, follow these steps:

1. Create a `VolumeSnapshotClass` and a `Secret`, containing the access key and secret key of S3 storage, if they were not created earlier (for example, if you are restoring data in a new cluster).

   Example of `VolumeSnapshotClass` and `Secret`:

   ```yaml
   kind: VolumeSnapshotClass
   apiVersion: snapshot.storage.k8s.io/v1
   metadata:
     name: linstor-csi-snapshot-class-s3
   driver: linstor.csi.linbit.com
   deletionPolicy: Retain
   parameters:
     snap.linstor.csi.linbit.com/type: S3
     snap.linstor.csi.linbit.com/remote-name: backup-remote               # Linstor remote name.   
     snap.linstor.csi.linbit.com/allow-incremental: "false"               # Whether to use incremental copies. 
     snap.linstor.csi.linbit.com/s3-bucket: snapshot-bucket               # The name of the S3 bucket, for data storage.
     snap.linstor.csi.linbit.com/s3-endpoint: s3.us-west-1.amazonaws.com  # S3 endpoint URL.
     snap.linstor.csi.linbit.com/s3-signing-region: us-west-1             # The name of the S3 region. 
     # Use virtual hosted–style or path-style S3 URL. 
     # https://docs.aws.amazon.com/AmazonS3/latest/userguide/VirtualHosting.html
     snap.linstor.csi.linbit.com/s3-use-path-style: "false"    
     # Secret, containing the access key and secret key of access to S3 bucket.
     csi.storage.k8s.io/snapshotter-secret-name: linstor-csi-s3-access
     csi.storage.k8s.io/snapshotter-secret-namespace: storage
   ---
   kind: Secret
   apiVersion: v1
   metadata:
     name: linstor-csi-s3-access
     namespace: storage
   immutable: true
   type: linstor.csi.linbit.com/s3-credentials.v1
   stringData:
     access-key: *!ACCESS_KEY*  # S3 access key.
     secret-key: *!SECRET_KEY*  # S3 secret key.
   ```

1. Get a snapshot id for recovery by using one the the following method:

   1. Get a list of snapshots, and select the one you need (the `SnapshotName` column):

      ```shell
      linstor backup list <backup-remote-name>
      ```

      , where the `<backup-remote-name>` is the remote name used in the `VolumeSnapshotClass`.

   1. Get a snapshot id from the object name in the S3 backet via the UI interface or CLI utilities of the S3 service.

1. Create a `VolumeSnapshotContent`, pointing to a specific snapshot id.

   > VolumeSnapshotContent is a cluster-wide resource. Each VolumeSnapshotClass can only be bound with one VolumeSnapshot. So make sure its name is unique.
   Example:

   ```yaml
   apiVersion: snapshot.storage.k8s.io/v1
   kind: VolumeSnapshotContent
   metadata:
     name: restored-snap-content-from-s3
   spec:
     deletionPolicy: Delete
     driver: linstor.csi.linbit.com
     source:
       snapshotHandle: *!snapshot_id*                        # Snapshot ID .  
     volumeSnapshotClassName: linstor-csi-snapshot-class-s3  # The name of the VolumeSnapshotClass, with access to the S3 storage.
     volumeSnapshotRef:
       apiVersion: snapshot.storage.k8s.io/v1
       kind: VolumeSnapshot
       name: example-backup-from-s3                          # The name of the VolumeSnapshot (will be created below).
       namespace: storage
   ```

1. Create a `VolumeSnapshot`, pointing to the created `VolumeSnapshotContent`.

   Example:

   ```yaml
   apiVersion: snapshot.storage.k8s.io/v1
   kind: VolumeSnapshot
   metadata:
     name: example-backup-from-s3
     namespace: storage
   spec:
     source:
       volumeSnapshotContentName: restored-snap-content-from-s3 # The name of the VolumeSnapshotContent created earlier.
     volumeSnapshotClassName: linstor-csi-snapshot-class-s3     # The name of the VolumeSnapshotClass, with access to the S3 storage.
   ```

1. Create a `PersistentVolumeClaim`.

   Example:

   ```yaml
   apiVersion: v1
   kind: PersistentVolumeClaim
   metadata:
     name: restored-data
     namespace: storage
   spec:
     storageClassName: "linstor-thindata-r1" # The name of the StorageClass.  
     dataSource:
       name: example-backup-from-s3          # The name of the VolumeSnapshot, created earlier.
       kind: VolumeSnapshot
       apiGroup: snapshot.storage.k8s.io
     accessModes:
       - ReadWriteOnce
     resources:
       requests:
         storage: 2Gi
   ```

Use the created `PersistentVolumeClaim` to access a copy of the recovered data.

### Scheduled Backup Shipping

LINSTOR supports automatic scheduled backups.
However, this feature is currently only available through the LINSTOR CLI.

To do this, you need to first create an S3 remote:

```bash
linstor remote create s3 myRemote s3.us-west-2.amazonaws.com \
  my-bucket us-west-2 admin password [--use-path-style]
```

After that, create a schedule and enable it for your remote.  
To do this, please refer to the [official LINSTOR documentation](https://linbit.com/drbd-user-guide/linstor-guide-1_0-en/#s-linstor-scheduled-backup-shipping)
