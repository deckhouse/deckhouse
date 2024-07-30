---
title: "Модуль snapshot-controller: примеры конфигурации"
---

### Использование снапшотов

Чтобы использовать снапшоты, необходимо указать конкретный `VolumeSnapshotClass`.
Чтобы получить список доступных VolumeSnapshotClass в вашем кластере, выполните:

```shell
kubectl get volumesnapshotclasses.snapshot.storage.k8s.io
```

Затем вы сможете использовать VolumeSnapshotClass для создания снапшота из существующего тома:

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

Спустя небольшой промежуток времени снапшот будет готов:

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

Вы можете восстановить содержимое этого снапшота, создав новый PVC. Для этого необходимо указать снапшот в качестве источника:

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

### Клонирование CSI-томов

Основываясь на концепции снапшотов, вы также можете осуществить клонирование Persistent Volumes, а точнее существующих PersistentVolumeClaims (PVC).
Однако спецификация CSI не позволяет производить клонирование томов в пространстве имен и StorageClass'ах, отличных от оригинального PVC
(обратитесь [к документации Kubernetes](https://kubernetes.io/docs/concepts/storage/volume-pvc-datasource/), чтобы узнать больше об ограничениях).

Чтобы клонировать том, создайте новый PVC и укажите исходный PVC в `dataSource`:

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
