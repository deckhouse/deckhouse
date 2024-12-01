---
title: "CEPH-хранилище"
permalink: ru/virtualization-platform/documentation/admin/platform-management/storage/sds/ceph.html
lang: ru
---

Чтобы создать объекты StorageClass на основе RBD (RADOS Block Device) или файловой системы Ceph, можно использовать модуль csi-ceph, который позволяет настроить подключение к одному или нескольким Ceph-кластерам.

## Включение модуля

Чтобы включить модуль csi-ceph, примените ресурс `ModuleConfig`:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-ceph
spec:
  enabled: true
EOF
```

## Подключение к Ceph-кластеру

Чтобы настроить подключение к Ceph-кластеру, необходимо применить ресурс `CephClusterConnection`. Пример команды:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: CephClusterConnection
metadata:
  name: ceph-cluster-1
spec:
  # FSID/UUID Ceph-кластера.
  # Получить FSID/UUID Ceph-кластера можно с помощью команды `ceph fsid`.
  clusterID: 2bf085fc-5119-404f-bb19-820ca6a1b07e
  # Список IP-адресов ceph-mon’ов в формате 10.0.0.10:6789.
  monitors:
    - 10.0.0.10:6789
EOF
```

Проверить создание подключения можно командой (фаза должна быть в статусе `Created`):

```shell
d8 k get cephclusterconnection ceph-cluster-1
```

## Аутентификация

Чтобы пройти аутентификацию в Ceph-кластере, необходимо определить параметры аутентификации в ресурсе `CephClusterAuthentication`:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: CephClusterAuthentication
metadata:
  name: ceph-auth-1
spec:
  # Имя пользователя без `client.`.
  userID: admin
  # Ключ авторизации, соответствующий userID.
  userKey: AQDbc7phl+eeGRAAaWL9y71mnUiRHKRFOWMPCQ==
EOF
```

## Создание StorageClass

Создание объектов StorageClass осуществляется через ресурс `CephStorageClass`, который определяет конфигурацию для желаемого класса хранения. Ручное создание ресурса StorageClass без `CephStorageClass` может привести к ошибкам.

Пример создания StorageClass на основе RBD (RADOS Block Device):

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: CephStorageClass
metadata:
  name: ceph-rbd-sc
spec:
  clusterConnectionName: ceph-cluster-1
  clusterAuthenticationName: ceph-auth-1
  reclaimPolicy: Delete
  type: RBD
  rbd:
    defaultFSType: ext4
    pool: ceph-rbd-pool
EOF
```

Пример создания StorageClass на основе файловой системы Ceph:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: CephStorageClass
metadata:
  name: ceph-fs-sc
spec:
  clusterConnectionName: ceph-cluster-1
  clusterAuthenticationName: ceph-auth-1
  reclaimPolicy: Delete
  type: CephFS
  cephFS:
    fsName: cephfs
EOF
```

Проверьте, что созданные ресурсы `CephStorageClass` перешли в состояние `Created` и соответствующие объекты StorageClass создались:

```shell
d8 k get cephstorageclass

# NAME          PHASE     AGE
# ceph-rbd-sc   Created   1h
# ceph-fs-sc    Created   1h
```

Созданный StorageClass можно проверить с помощью команды:

```shell
d8 k get sc

# NAME          PROVISIONER        RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
# ceph-rbd-sc   rbd.csi.ceph.com   Delete          WaitForFirstConsumer   true                   15s
# ceph-fs-sc    rbd.csi.ceph.com   Delete          WaitForFirstConsumer   true                   15s
```

Если объекты StorageClass появились, значит настройка модуля csi-ceph завершена.
Теперь пользователи могут создавать PersistentVolume, указывая созданные объекты StorageClass.
