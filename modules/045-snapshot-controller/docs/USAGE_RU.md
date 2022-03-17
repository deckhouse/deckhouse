---
title: "Модуль snapshot-controller: примеры конфигурации"
---

### Использование снапшотов

Чтобы использовать снапшоты, сначала необходимо создать `VolumeSnapshotClass`:

```yaml
apiVersion: snapshot.storage.k8s.io/v1beta1
kind: VolumeSnapshotClass
metadata:
  name: my-first-linstor-snapshot-class
driver: linstor.csi.linbit.com
deletionPolicy: Delete
```

Затем вы сможете использовать этот snapshot class для создания снапшота из существующего тома:

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

Спустя небольшой промежуток времени снапшот будет готов: 

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

Вы можете восстановить содержимое этого снапшоты, создав новый PVC указав снапшот в качестве источника: 

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

### Клонирование CSI-томов

Основываясь на концепции снапшотов, вы также можете осуществить клонирование persistent volumes, а точнее существующих persistent volume claims (PVC).
Однако спецификация CSI не позволяет производить клонирование томов в неймспейсах и созданных со `StorageClass` отличным от оригинального PVC.
(обратитесь к [документации Kubernetes](https://kubernetes.io/docs/concepts/storage/volume-pvc-datasource/) чтобы узнать больше об ограничениях).

Чтобы клонировать том, создайте новый PVC и укажите исходный PVC в `dataSource`:

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
