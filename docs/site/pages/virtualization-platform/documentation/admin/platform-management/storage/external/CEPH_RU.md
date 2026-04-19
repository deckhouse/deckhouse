---
title: "Распределённое хранилище Ceph"
permalink: ru/virtualization-platform/documentation/admin/platform-management/storage/external/ceph.html
lang: ru
---

Ceph — это масштабируемая распределённая система хранения, обеспечивающая высокую доступность и отказоустойчивость данных. В Deckhouse Virtualization Platform (DVP) поддерживается интеграция с Ceph-кластерами. Это даёт возможность динамически управлять хранилищем и использовать StorageClass на основе RADOS Block Device (RBD) или CephFS.

На этой странице представлены инструкции по подключению Ceph в DVP, настройке аутентификации, созданию объектов StorageClass, а также проверке работоспособности хранилища.

{% alert level="warning" %}
При переключении на данный модуль с модуля `ceph-csi` производится автоматическая миграция, но ее запуск требует подготовки:

1. Выполните scale всех операторов (redis, clickhouse, kafka и т.д) в ноль реплик. В момент миграции операторы в кластере работать не должны. Единственное исключение — оператор `prometheus` в составе DVP, который будет автоматически отключен в процессе миграции.
1. Выключите модуль `ceph-csi` и включите модуль `csi-ceph`.
1. В логах DVP дождитесь окончания процесса миграции («Finished migration from Ceph CSI module»).
1. Создайте тестовые VM/PVC для проверки работоспособности CSI.
1. Верните операторы в работоспособное состояние.
   При наличии в ресурсах CephCSIDriver поля `spec.cephfs.storageClasses.pool` отличного от `cephfs_data` миграция будет завершаться с ошибкой.
   При наличии Ceph StorageClass, созданного не с помощью ресурса CephCSIDriver, потребуется ручная миграция.
   В этих случаях необходимо связаться [с технической поддержкой](https://deckhouse.ru/tech-support/).
   {% endalert %}

## Включение модуля

Для подключения Ceph-кластера в DVP необходимо включить модуль `csi-ceph`. Для этого примените ресурс ModuleConfig:

```shell
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

Чтобы настроить подключение к Ceph-кластеру, примените ресурс [CephClusterConnection](/modules/csi-ceph/stable/cr.html#cephclusterconnection). Пример команды:

```shell
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: CephClusterConnection
metadata:
  name: ceph-cluster-1
spec:
  # FSID/UUID Ceph-кластера.
  # Получить FSID/UUID Ceph-кластера можно с помощью команды `ceph fsid`.
  clusterID: 2bf085fc-5119-404f-bb19-820ca6a1b07e
  # Список IP-адресов ceph-mon’ов в формате `10.0.0.10:6789`.
  monitors:
    - 10.0.0.10:6789
  # Имя пользователя без `client.`.
  # Получить имя пользователя можно с помощью команды `ceph auth list`.
  userID: admin
  # Ключ авторизации, соответствующий userID.
  # Получить ключ авторизации можно с помощью команды `ceph auth get-key client.admin`.
  userKey: AQDiVXVmBJVRLxAAg65PhODrtwbwSWrjJwssUg==
EOF
```

Проверьте создание подключения следующей командой (`Phase` должен быть `Created`):

```shell
d8 k get cephclusterconnection ceph-cluster-1
```

## Создание StorageClass

Создание объектов StorageClass осуществляется через ресурс [CephStorageClass](/modules/csi-ceph/stable/cr.html#cephstorageclass), который определяет конфигурацию для желаемого класса хранения. Ручное создание ресурса StorageClass без [CephStorageClass](/modules/csi-ceph/stable/cr.html#cephstorageclass) может привести к ошибкам. Пример создания StorageClass на основе RBD:

```shell
d8 k apply -f - <<EOF
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
EOF
```

Пример создания StorageClass на основе файловой системы Ceph:

```shell
d8 k apply -f - <<EOF
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
EOF
```

Проверьте, что созданные ресурсы [CephStorageClass](/modules/csi-ceph/stable/cr.html#cephstorageclass) перешли в состояние `Created`, выполнив следующую команду:

```shell
d8 k get cephstorageclass
```

В результате будет выведена информация о созданных ресурсах [CephStorageClass](/modules/csi-ceph/stable/cr.html#cephstorageclass):

```console
NAME          PHASE     AGE
ceph-rbd-sc   Created   1h
ceph-fs-sc    Created   1h
```

Проверьте созданный StorageClass с помощью следующей команды:

```shell
d8 k get sc
```

В результате будет выведена информация о созданном StorageClass:

```console
NAME          PROVISIONER        RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
ceph-rbd-sc   rbd.csi.ceph.com   Delete          WaitForFirstConsumer   true                   15s
ceph-fs-sc    rbd.csi.ceph.com   Delete          WaitForFirstConsumer   true                   15s
```

Если объекты StorageClass появились, значит настройка модуля `csi-ceph` завершена. Теперь пользователи могут создавать PersistentVolume, указывая созданные объекты StorageClass.

## Получение списка томов RBD, смонтированных на каждом узле

Для получения списка томов RBD, смонтированных на каждом узле кластера, выполните следующую команду:

```shell
d8 k -n d8-csi-ceph get po -l app=csi-node-rbd -o custom-columns=NAME:.metadata.name,NODE:.spec.nodeName --no-headers \
  | awk '{print "echo "$2"; kubectl -n d8-csi-ceph exec  "$1" -c node -- rbd showmapped"}' | bash

```

## Поддерживаемые версии Ceph

- Официальная поддержка — Ceph версии 16.2.0 и выше.
- Совместимость — решение работает с кластерами Ceph версии 14.2.0 и выше, однако рекомендуется обновить Ceph до версии 16.2.0 или выше для обеспечения максимальной стабильности и доступа к последним исправлениям.

## Поддерживаемые режимы доступа к томам

- RBD — ReadWriteOnce (RWO) — доступ к блочному тому возможен только с одного узла.
- CephFS — ReadWriteOnce (RWO) и ReadWriteMany (RWX) — одновременный доступ к файловой системе с нескольких узлов.

## Примеры

Пример описания [CephClusterConnection](/modules/csi-ceph/stable/cr.html#cephclusterconnection):

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

Проверить создание объекта можно следующей командой (`Phase` должен быть `Created`):

```shell
d8 k get cephclusterconnection <имя cephclusterconnection>
```

Пример описания [CephStorageClass](/modules/csi-ceph/stable/cr.html#cephstorageclass):

- Для RBD

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

- Для CephFS:

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

Проверить создание объекта можно следующей командой (`Phase` должен быть `Created`):

```shell
d8 k get cephstorageclass <имя storage class>
```
