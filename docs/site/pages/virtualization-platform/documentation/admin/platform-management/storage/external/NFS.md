---
title: "NFS Storage"
permalink: en/virtualization-platform/documentation/admin/platform-management/storage/sds/nfs.html
---

To manage volumes based on the NFS (Network File System) protocol, you can use the `csi-nfs` module, which allows the creation of a StorageClass through the creation of custom `NFSStorageClass` resources.

## Enabling the module

To enable the `csi-nfs` module, apply the following resource:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-nfs
spec:
  enabled: true
  version: 1
EOF
```

Wait until the `csi-nfs` module transitions to the `Ready` state:

```shell
d8 k get module csi-nfs -w

# NAME      WEIGHT   STATE     SOURCE     STAGE   STATUS
# csi-nfs   910      Enabled   Embedded           Ready
```

## Creating the StorageClass

To create a StorageClass, you need to use the `NFSStorageClass` resource. Manually creating a StorageClass without `NFSStorageClass` may lead to errors.

Example of creating a StorageClass based on NFS:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: NFSStorageClass
metadata:
  name: nfs-storage-class
spec:
  connection:
    # Address of the NFS server.
    host: 10.223.187.3
    # Path to the mount point on the NFS server.
    share: /
    # Version of the NFS server.
    nfsVersion: "4.1"
  # Reclaim policy when deleting PVC.
  # Allowed values:
  # - Delete (PVC deletion will also delete PV and data on the NFS server).
  # - Retain (PVC deletion will not delete PV or data on the NFS server, requiring manual removal by the user).
  # [Learn more...](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#reclaiming)
  reclaimPolicy: Delete
  # Volume creation mode.
  # Allowed values: "Immediate", "WaitForFirstConsumer".
  # [Learn more...](https://kubernetes.io/docs/concepts/storage/storage-classes/#volume-binding-mode)
  volumeBindingMode: WaitForFirstConsumer
EOF
```

Check that the created `NFSStorageClass` resource has transitioned to the `Created` state, and the corresponding StorageClass has been created:

```shell
d8 k get NFSStorageClass nfs-storage-class -w

# NAME                PHASE     AGE
# nfs-storage-class   Created   1h

d8 k get sc nfs-storage-class

# NAME                PROVISIONER      RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
# nfs-storage-class   nfs.csi.k8s.io   Delete          WaitForFirstConsumer   true                   1h
```

If the StorageClass named `nfs-storage-class` appears, it means the `csi-nfs` module has been configured successfully. Users can now create PersistentVolumes by specifying the `nfs-storage-class` StorageClass.

For each `PersistentVolume` resource, a directory `<share-directory>/<PersistentVolume-name>` will be created.

## Checking module functionality

To check the functionality of the `csi-nfs` module, you need to verify the pod statuses in the `d8-csi-nfs` namespace.
All pods should be in the `Running` or `Completed` state, and the `csi-nfs` pods should be running on all nodes.

You can check the module's functionality with the following command:

```shell
d8 k -n d8-csi-nfs get pod -owide -w

# NAME                             READY   STATUS    RESTARTS   AGE   IP             NODE       NOMINATED NODE   READINESS GATES
# controller-547979bdc7-5frcl      1/1     Running   0          1h    10.111.2.84    master     <none>           <none>
# csi-controller-5c6bd5c85-wzwmk   6/6     Running   0          1h    172.18.18.50   master     <none>           <none>
# webhooks-7b5bf9dbdb-m5wxb        1/1     Running   0          1h    10.111.0.16    master     <none>           <none>
# csi-nfs-8mpcd                    2/2     Running   0          1h    172.18.18.50   master     <none>           <none>
# csi-nfs-n6sks                    2/2     Running   0          1h    172.18.18.51   worker-1   <none>           <none>
```