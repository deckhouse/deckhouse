---
title: "The snapshot-controller moule: configuration examples"
---

### Using snapshots

To use snapshots, you first need to create a `VolumeSnapshotClass`:

```yaml
apiVersion: snapshot.storage.k8s.io/v1beta1
kind: VolumeSnapshotClass
metadata:
  name: my-first-linstor-snapshot-class
driver: linstor.csi.linbit.com
deletionPolicy: Delete
```

You can then use this snapshot class to create a snapshot from an existing PVC:

```yaml
apiVersion: snapshot.storage.k8s.io/v1beta1
kind: VolumeSnapshot
metadata:
  name: my-first-linstor-snapshot
spec:
  volumeSnapshotClassName: my-first-linstor-snapshot-class
  source:
    persistentVolumeClaimName: my-first-linstor-volume
```

After a short wait, the snapshot will be ready:

```yaml
$ kubectl describe volumesnapshots.snapshot.storage.k8s.io my-first-linstor-snapshot
...
Spec:
  Source:
    Persistent Volume Claim Name:  my-first-linstor-snapshot
  Volume Snapshot Class Name:      my-first-linstor-snapshot-class
Status:
  Bound Volume Snapshot Content Name:  snapcontent-b6072ab7-6ddf-482b-a4e3-693088136d2c
  Creation Time:                       2020-06-04T13:02:28Z
  Ready To Use:                        true
  Restore Size:                        500Mi
```

You can restore the content of this snaphost by creating a new PVC with the snapshot as source:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-first-linstor-volume-from-snapshot
spec:
  storageClassName: linstor-basic-storage-class
  dataSource:
    name: my-first-linstor-snapshot
    kind: VolumeSnapshot
    apiGroup: snapshot.storage.k8s.io
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 500Mi
```

### CSI Volume Cloning

Based on the concept of snapshots you can also perform cloning of persistent volumes - or to be more precise: of existing persistent volume claims (PVC).
However the CSI specification mentions some restrictions regarding cloning PVCs in different namespace and storage classes than original PVC
(see [Kubernetes documentation](https://kubernetes.io/docs/concepts/storage/volume-pvc-datasource/) for details).

To clone a volume create a new PVC and define the origin PVC in the dataSource:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-cloned-pvc
spec:
  storageClassName: linstor-basic-storage-class
  dataSource:
    name: my-origin-linstor-pvc
    kind: PersistentVolumeClaim
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 500Mi
```
