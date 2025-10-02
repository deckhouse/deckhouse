---
title: "Модуль csi-ceph: примеры"
---

## Пример описания `CephClusterConnection`

```yaml
apiVersion: storage.deckhouse.io/v1alpha1
kind: CephClusterConnection
metadata:
  name: ceph-cluster-1
spec:
  clusterID: 0324bfe8-c36a-4829-bacd-9e28b6480de9
  monitors:
  - 172.20.1.28:6789
  - 172.20.1.34:6789
  - 172.20.1.37:6789
  userID: admin
  userKey: AQDiVXVmBJVRLxAAg65PhODrtwbwSWrjJwssUg==
```

- Проверить создание объекта можно командой (Phase должен быть `Created`):

```shell
kubectl get cephclusterconnection <имя cephclusterconnection>
```

## Пример описания `CephStorageClass`

### RBD

```yaml
apiVersion: storage.deckhouse.io/v1alpha1
kind: CephStorageClass
metadata:
  name: ceph-rbd-sc
spec:
  clusterConnectionName: ceph-cluster-1
  reclaimPolicy: Delete
  type: RBD
  rbd:
    defaultFSType: ext4
    pool: ceph-rbd-pool  
```

### CephFS

```yaml
apiVersion: storage.deckhouse.io/v1alpha1
kind: CephStorageClass
metadata:
  name: ceph-fs-sc
spec:
  clusterConnectionName: ceph-cluster-1
  reclaimPolicy: Delete
  type: CephFS
  cephFS:
    fsName: cephfs
```

### Проверить создание объекта можно командой (Phase должен быть `Created`):

```shell
kubectl get cephstorageclass <имя storage class>
```
