---
title: "Модуль csi-ceph"
---

{% alert level="warning" %}
При переключении на данный модуль с модуля ceph-csi производится автоматическая миграция, но ее запуск требует подготовки:
1. Необходимо сделать scale всех операторов (redis, clickhouse, kafka и т.д) в ноль реплик, в момент миграции операторы в кластере работать не должны. Единственное исключение - оператор prometheus в составе Deckhouse, в процессе миграции его отключит автоматически
2. Выключить модуль ceph-csi и включить модуль csi-ceph
3. В логах Deckhouse дождаться окончания процесса миграции (Finished migration from Ceph CSI module)
4. Создать тестовые pod/pvc для проверки работоспособности CSI
5. Вернуть операторы в работоспособное состояние
При наличии Ceph StorageClass, созданного не с помощью ресурса CephCSIDriver потребуется ручная миграция.
В этом случае необходимо связаться с техподдержкой.
{% endalert %}

{% alert level="info" %}
Для работы с снапшотами требуется подключенный модуль [snapshot-controller](../snapshot-controller/).
{% endalert %}

Ceph — это масштабируемая распределённая система хранения, обеспечивающая высокую доступность и отказоустойчивость данных. В Deckhouse поддерживается интеграция с Ceph-кластерами, что позволяет динамически управлять хранилищем и использовать StorageClass на основе RBD (RADOS Block Device) или CephFS.

На этой странице представлены инструкции по подключению Ceph в Deckhouse, настройке аутентификации, созданию объектов StorageClass, а также проверке работоспособности хранилища.

## Включение модуля

Для подключения Ceph-кластера в Deckhouse необходимо включить модуль `csi-ceph`. Для этого примените ресурс ModuleConfig:

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

Чтобы настроить подключение к Ceph-кластеру, необходимо применить ресурс [CephClusterConnection](cr.html#cephclusterconnection). Пример команды:

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
  # Имя пользователя без `client.`.
  # Получить имя пользователя можно с помощью команды `ceph auth list`.
  userID: admin
  # Ключ авторизации, соответствующий userID.
  # Получить ключ авторизации можно с помощью команды `ceph auth get-key client.admin`.
  userKey: AQDiVXVmBJVRLxAAg65PhODrtwbwSWrjJwssUg==
EOF
```

Проверить создание подключения можно командой (фаза должна быть в статусе `Created`):

```shell
d8 k get cephclusterconnection ceph-cluster-1
```

## Создание StorageClass

Создание объектов StorageClass осуществляется через ресурс [CephStorageClass](cr.html#cephstorageclass), который определяет конфигурацию для желаемого класса хранения. Ручное создание ресурса StorageClass без [CephStorageClass](cr.html#cephstorageclass) может привести к ошибкам. Пример создания StorageClass на основе RBD (RADOS Block Device):

```yaml
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

```yaml
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

Проверьте, что созданные ресурсы [CephStorageClass](cr.html#cephstorageclass) перешли в состояние `Created`, выполнив следующую команду:

```shell
d8 k get cephstorageclass
```

В результате будет выведена информация о созданных ресурсах [CephStorageClass](cr.html#cephstorageclass):

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
