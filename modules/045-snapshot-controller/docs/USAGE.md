---
title: "The snapshot-controller module: configuration examples"
---

### Using snapshots

To use snapshots, you need to specify a `VolumeSnapshotClass`.
To get a list of available VolumeSnapshotClasses in your cluster, run:

```shell
kubectl get volumesnapshotclasses.snapshot.storage.k8s.io
```

You can then use VolumeSnapshotClass to create a snapshot from an existing PVC:

```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: my-first-snapshot
spec:
  volumeSnapshotClassName: sds-replicated-volume
  source:
    persistentVolumeClaimName: my-first-volume
```

After a short wait, the snapshot will be ready:

```yaml
$ kubectl describe volumesnapshots.snapshot.storage.k8s.io my-first-snapshot
...
Spec:
  Source:
    Persistent Volume Claim Name:  my-first-snapshot
  Volume Snapshot Class Name:      sds-replicated-volume
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
  name: my-first-volume-from-snapshot
spec:
  storageClassName: sds-replicated-volume-data-r2
  dataSource:
    name: my-first-snapshot
    kind: VolumeSnapshot
    apiGroup: snapshot.storage.k8s.io
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 500Mi
```

### CSI Volume Cloning

Based on the concept of snapshots, you can also perform cloning of persistent volumes - or, more precisely, existing persistent volume claims (PVC).
However, the CSI specification mentions some restrictions regarding cloning PVCs in different namespace and storage classes than the original PVC
(see [Kubernetes documentation](https://kubernetes.io/docs/concepts/storage/volume-pvc-datasource/) for details).

To clone a volume create a new PVC and define the origin PVC in the dataSource:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-cloned-pvc
spec:
  storageClassName: sds-replicated-volume-data-r2
  dataSource:
    name: my-origin-pvc
    kind: PersistentVolumeClaim
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 500Mi
```
