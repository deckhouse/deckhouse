---
title: "Configuring volume snapshot creation"
permalink: en/admin/storage/snapshot-controller.html
---

Deckhouse Kubernetes Platform supports volume snapshot creation for CSI drivers in a Kubernetes cluster.

Snapshots capture the state of a volume at a specific point in time and can be used for data recovery or volume cloning. The ability to create snapshots depends on the capabilities of the CSI driver in use.

## Supported CSI Drivers

The following CSI drivers support snapshot creation:

- [OpenStack Cloud Provider](#TODO);
- [VMware vSphere Cloud Provider](#TODO);
- [Distributed storage based on Ceph](../storage/external/ceph.html);
- [Amazon Web Services Cloud Provider](#TODO);
- [Microsoft Azure Cloud Provider](#TODO);
- [Google Cloud Platform Cloud Provider](#TODO);
- [Replicated storage based on DRBD](../storage/sds/lvm-replicated.html);
- [NFS-based storage](../storage/external/nfs.html).

## Creating Snapshots

Before creating snapshots, make sure that [VolumeSnapshotClass](../../reference/cr/volumesnapshotclass/) resources are configured in the cluster. You can list available classes with the following command:

```shell
d8 k get volumesnapshotclasses.snapshot.storage.k8s.io
```

To create a snapshot for a volume, specify the required [VolumeSnapshotClass](../../reference/cr/volumesnapshotclass/) in the manifest:

```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: example-snapshot
spec:
  volumeSnapshotClassName: <class-name>
  source:
    persistentVolumeClaimName: <pvc-name>
```

## Restoring from a Snapshot

To restore data from a snapshot, create a PVC that references the existing [VolumeSnapshot](../../reference/cr/volumesnapshot/):

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: restored-pvc
spec:
  dataSource:
    name: example-snapshot
    kind: VolumeSnapshot
    apiGroup: snapshot.storage.k8s.io
  storageClassName: <storage-class-name>
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
```

{% alert level=“warning” %}
Not all CSI drivers support restoring volumes from snapshots. Ensure that your driver provides the required capabilities.
{% endalert %}
