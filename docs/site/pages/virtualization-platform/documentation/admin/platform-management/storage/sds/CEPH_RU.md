---
title: "CEPH-хранилище"
permalink: ru/virtualization-platform/documentation/admin/platform-management/storage/sds/ceph.html
lang: ru
---

## TODO: Описать процесс создания снапшота (не для настроек CSI поддерживается создание снапшотов).
## TODO: Тут описан внутренний модуль ceph-csi. Есть еще и внешний, позже сравню их, вероятно еще что-то будет принесено оттуда.

Чтобы создать StorageClass’ы на базе RBD или файловой системы Ceph, можно использовать модуль ceph-csi, 
который позволяет настроить подключение к одному или нескольким Ceph-кластерам.

Чтобы включить модуль ceph-csi, примените следующий ресурс `ModuleConfig`:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: ceph-csi
spec:
  enabled: true
EOF
```

Чтобы подключить Ceph-кластер и настроить StorageClass’ы, примените следующий ресурс `CephCSIDriver`:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: CephCSIDriver
metadata:
  name: example
spec:
  # FSID/UUID Ceph-кластера.
  # Получить FSID/UUID Ceph-кластера можно с помощью команды `ceph fsid`.
  clusterID: 2bf085fc-5119-404f-bb19-820ca6a1b07e
  # Список IP-адресов ceph-mon’ов в формате 10.0.0.10:6789.
  monitors:
    - 10.0.0.10:6789
  # Имя пользователя без `client.`.
  userID: admin
  # Ключ авторизации, соответствующий userID.
  userKey: AQDbc7phl+eeGRAAaWL9y71mnUiRHKRFOWMPCQ==
  rbd:
    # Описание StorageClass’ов для Rados Block Device (RBD).
    storageClasses:
        # Включает возможность изменять размер тома, редактируя соответствующий объект PersistentVolumeClaim.
        # По умолчанию: true.
        # [Подробнее...](https://kubernetes.io/docs/concepts/storage/storage-classes/#allow-volume-expansion)
      - allowVolumeExpansion: true
        #  Файловая система по умолчанию для создаваемых Persistent Volumes. 
        # По умолчанию: "ext4".
        # Допустимые значения: "ext4", "xfs".
        defaultFSType: ext4
        # Список опций монтирования.
        mountOptions:
          - discard
        # Суфикс, который будет использован для созданного StorageClass’а.
        # В качестве первой части используется имя созданного CephCSIDriver.
        namePostfix: csi-rbd
        # Название пула, в котором будут создаваться RBD-образы.
        pool: kubernetes-rbd
        # Политика возврата для Persistent Volume.
        # По умолчанию: "Retain"
        # Допустимые значения: "Delete", "Retain"
        # [Подробнее...](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#reclaiming)
        reclaimPolicy: Delete
  cephfs:
    # Описание StorageClass’ов для CephFS.
    storageClasses:
        # Включает возможность изменять размер тома, редактируя соответствующий объект PersistentVolumeClaim.
        # По умолчанию true.
        # [Подробнее...](https://kubernetes.io/docs/concepts/storage/storage-classes/#allow-volume-expansion)
      - allowVolumeExpansion: true
        # Имя файловой системы CephFS.
        fsName: cephfs
        # Суфикс, который будет использован для созданного StorageClass’а.
        # В качестве первой части используется имя созданного CephCSIDriver.
        namePostfix: csi-cephfs
        # Название пула, в котором будут создаваться RBD-образы.
        pool: cephfs_data
        # Политика возврата для Persistent Volume.
        # По умолчанию: "Retain"
        # Допустимые значения: "Delete", "Retain"
        # [Подробнее...](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#reclaiming)
        reclaimPolicy: Delete
EOF
```

Созданные StorageClass'ы можно проверить с помощью команды:
```yaml
d8 k get StorageClass

# NAME                 PROVISIONER        RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
# example-csi-rbd      rbd.csi.ceph.com   Delete          WaitForFirstConsumer   true                   15s
# example-csi-cephfs   rbd.csi.ceph.com   Delete          WaitForFirstConsumer   true                   15s
```

Модуль ceph-csi позволяет подключить несколько Ceph-кластеров. Для каждого подключения нужно создать соответсвующий ресурс `CephCSIDriver`.
