---
title: "Распределённое хранилище Ceph"
permalink: ru/admin/configuration/storage/external/ceph.html
description: "Настройка интеграции с распределённым хранилищем Ceph в Deckhouse Kubernetes Platform. Настройка RBD и CephFS, конфигурация аутентификации и управление высокодоступным хранилищем."
lang: ru
---

Ceph — это масштабируемая распределённая система хранения с высокой доступностью и отказоустойчивостью. Deckhouse Kubernetes Platform (DKP) обеспечивает интеграцию Ceph-кластера при помощи модуля [`csi-ceph`](/modules/csi-ceph/). Это даёт возможность динамически управлять хранилищем и использовать StorageClass на основе RADOS Block Device (RBD) или CephFS.

На этой странице представлены инструкции по подключению Ceph в Deckhouse, настройке аутентификации, созданию объектов StorageClass, а также проверке работоспособности хранилища.

{% alert level="info" %}
Для работы со снимками требуется подключенный модуль [snapshot-controller](/modules/snapshot-controller/).
{% endalert %}

## Миграция с модуля `ceph-csi`

При переключении на модуль `csi-ceph` с модуля `ceph-csi` выполняется автоматическая миграция, но её запуск требует предварительной подготовки:

1. Установите количество реплик в ноль для всех операторов (redis, clickhouse, kafka и др.). Исключение: оператор `prometheus` будет отключен автоматически.

1. Отключите модуль `ceph-csi` и [включите](#подключение-к-ceph-кластеру) `csi-ceph`.

1. Дождитесь завершения операции. В логах Deckhouse должно появиться сообщение "Finished migration from Ceph CSI module".

1. Проверьте работоспособность. Для этого создайте тестовые поды и PVC для проверки CSI.

1. Верните операторы в рабочее состояние.

{% alert level="warning" %}
**Примечание:** Если Ceph StorageClass был создан не через ресурс CephCSIDriver, потребуется ручная миграция. Обратитесь в техподдержку.
{% endalert %}

## Подключение к Ceph-кластеру

Для подключения Ceph-кластера следуйте пошаговым инструкциям ниже. Все команды выполняйте на машине с административным доступом к API Kubernetes.

1. Выполните команду для активации модуля `csi-ceph`:

   ```shell
   d8 s module enable csi-ceph
   ```

1. Дождитесь перехода модуля в состояние `Ready`:

   ```shell
   d8 k get module csi-ceph -w
   ```

1. Убедитесь, что все поды в пространстве имён `d8-csi-ceph` находятся в состоянии `Running` или `Completed` и развернуты на всех узлах кластера:

   ```shell
   d8 k -n d8-csi-ceph get pod -owide -w
   ```

1. Для настройки подключения к Ceph-кластеру примените ресурс [CephClusterConnection](/modules/csi-ceph/cr.html#cephclusterconnection).

   Пример команды:

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

1. Проверьте создание подключения командой (`Phase` должна быть в статусе `Created`):

   ```shell
   d8 k get cephclusterconnection ceph-cluster-1
   ```

1. Создайте объект StorageClass при помощи ресурса [CephStorageClass](/modules/csi-ceph/cr.html#cephstorageclass). Ручное создание StorageClass без использования [CephStorageClass](/modules/csi-ceph/cr.html#cephstorageclass) может привести к ошибкам.

   Пример создания StorageClass на основе RBD:

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

1. Проверьте, что созданные ресурсы [CephStorageClass](/modules/csi-ceph/cr.html#cephstorageclass) перешли в состояние `Created`:

   ```shell
   d8 k get cephstorageclass
   ```

   В результате будет выведена информация о созданных ресурсах [CephStorageClass](/modules/csi-ceph/cr.html#cephstorageclass):

   ```console
   NAME          PHASE     AGE
   ceph-rbd-sc   Created   1h
   ceph-fs-sc    Created   1h
   ```

1. Проверьте созданный StorageClass:

   ```shell
   d8 k get sc
   ```

   В результате будет выведена информация о созданном StorageClass:

   ```console
   NAME          PROVISIONER        RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
   ceph-rbd-sc   rbd.csi.ceph.com   Delete          WaitForFirstConsumer   true                   15s
   ceph-fs-sc    rbd.csi.ceph.com   Delete          WaitForFirstConsumer   true                   15s
   ```

Настройка подключения к Ceph-кластеру завершена. Вы можете использовать созданный StorageClass для создания PersistentVolumeClaim в ваших приложениях.

## Дополнительная информация

### Получение списка томов RBD, разделенного по узлам

Для мониторинга и диагностики полезно знать, какие RBD-тома подключены к каждому узлу кластера. Следующая команда позволяет получить детальную информацию о маппинге томов:

```shell
d8 k -n d8-csi-ceph get po -l app=csi-node-rbd -o custom-columns=NAME:.metadata.name,NODE:.spec.nodeName --no-headers \
  | awk '{print "echo "$2"; kubectl -n d8-csi-ceph exec  "$1" -c node -- rbd showmapped"}' | bash
```

### Поддерживаемые версии Ceph кластеров

Модуль `csi-ceph` имеет определенные требования к версии Ceph-кластера для обеспечения совместимости и стабильной работы. Официально поддерживаются версии >= 16.2.0. Из практики текущая версия работает с кластерами версий >=14.2.0, но рекомендуется обновить Ceph до актуальной версии.

### Поддерживаемые режимы работы томов

Различные типы хранилища Ceph поддерживают разные режимы доступа к томам, что важно учитывать при планировании архитектуры приложений.

- **RBD** — поддерживает только ReadWriteOnce (RWO) — доступ к тому только с одного узла кластера.
- **CephFS** — поддерживает ReadWriteOnce (RWO) и ReadWriteMany (RWX) — одновременный доступ к тому с нескольких узлов кластера.

### Проверка состояния подключения к Ceph

Для диагностики проблем с хранилищем необходимо уметь проверять состояние подключения к Ceph-кластеру и созданных StorageClass.

Для проверки статуса подключения выполните команду:

```shell
d8 k get cephclusterconnection <имя-подключения>
```

Для проверки статуса StorageClass выполните команду:

```shell
d8 k get cephstorageclass <имя-storageclass>
```
