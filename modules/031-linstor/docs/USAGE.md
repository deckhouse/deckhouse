---
title: "The linstor module: configuration examples"
---

## Using the linstor scheduler

The linstor scheduler considers the placement of data in storage and tries to place Pods on nodes where data is available locally first.  

Specify the `schedulerName: linstor` parameter in the Pod description to use the `linstor` scheduler.

An example of such a Pod:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: busybox
  namespace: default
spec:
  schedulerName: linstor # Using the linstor scheduler
  containers:
  - name: busybox
    image: busybox
    command: ["tail", "-f", "/dev/null"]
    volumeMounts:
    - name: my-first-linstor-volume
      mountPath: /data
    ports:
    - containerPort: 80
  volumes:
  - name: my-first-linstor-volume
    persistentVolumeClaim:
      claimName: "test-volume"
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
kubectl exec -n d8-linstor deploy/linstor-controller -- linstor snapshot list
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
      kubectl exec -n d8-linstor deploy/linstor-controller -- linstor backup list <backup-remote-name>
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
