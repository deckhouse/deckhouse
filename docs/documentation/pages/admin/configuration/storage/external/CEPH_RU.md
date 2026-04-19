---
title: "Распределённое хранилище Ceph"
permalink: ru/admin/configuration/storage/external/ceph.html
description: "Настройка интеграции с распределённым хранилищем Ceph в Deckhouse Kubernetes Platform. Настройка RBD и CephFS, конфигурация аутентификации и управление высокодоступным хранилищем."
lang: ru
---

Ceph — это масштабируемая распределённая система хранения с высокой доступностью и отказоустойчивостью. Deckhouse Kubernetes Platform (DKP) обеспечивает интеграцию Ceph-кластера при помощи модуля `csi-ceph`. Это даёт возможность динамически управлять хранилищем и использовать StorageClass на основе RADOS Block Device (RBD) или CephFS.

На этой странице представлены инструкции по подключению Ceph в Deckhouse, настройке аутентификации, созданию объектов StorageClass, а также проверке работоспособности хранилища.

{% alert level="info" %}
Для работы со снимками требуется подключенный модуль [`snapshot-controller`](/modules/snapshot-controller/).
{% endalert %}

## Миграция с модуля `ceph-csi`

При переключении на модуль `csi-ceph` с модуля `ceph-csi` выполняется автоматическая миграция, но её запуск требует предварительной подготовки:

1. Установите количество реплик в ноль для всех операторов (redis, clickhouse, kafka и др.). Исключение: оператор `prometheus` будет отключён автоматически.

1. Отключите модуль `ceph-csi` и [включите `csi-ceph`](#подключение-к-ceph-кластеру).

1. Дождитесь завершения операции. В логах DKP должно появиться сообщение "Finished migration from Ceph CSI module".

1. Проверьте работоспособность. Для этого создайте тестовые поды и PVC для проверки CSI.

1. Верните операторы в рабочее состояние (установите количество реплик обратно).

{% alert level="warning" %}
Если Ceph StorageClass был создан не через ресурс CephCSIDriver, потребуется ручная миграция. Обратитесь в техподдержку.
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

1. Убедитесь, что все поды в пространстве имён `d8-csi-ceph` находятся в состоянии `Running` или `Completed` и развёрнуты на всех узлах кластера:

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
     # Список IP-адресов ceph-mon в формате 10.0.0.10:6789.
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

### Получение списка томов RBD, разделённого по узлам

Для мониторинга и диагностики полезно знать, какие RBD-тома подключены к каждому узлу кластера. Следующая команда позволяет получить детальную информацию о маппинге томов:

```shell
d8 k -n d8-csi-ceph get po -l app=csi-node-rbd -o custom-columns=NAME:.metadata.name,NODE:.spec.nodeName --no-headers \
  | awk '{print "echo "$2"; kubectl -n d8-csi-ceph exec  "$1" -c node -- rbd showmapped"}' | bash
```

### Поддерживаемые версии Ceph-кластеров

Модуль `csi-ceph` предъявляет требования к версии Ceph-кластера, чтобы обеспечить совместимость и стабильную работу. Официально поддерживаются версии Ceph >= 16.2.0. На практике текущая версия модуля обычно работает и с кластерами >= 14.2.0, но для надёжной эксплуатации рекомендуется обновить Ceph до актуальной поддерживаемой версии.

### Поддерживаемые режимы работы томов

Различные типы хранилища Ceph поддерживают разные режимы доступа к томам, что важно учитывать при планировании архитектуры приложений.

- **RBD** — поддерживает только ReadWriteOnce (RWO) — доступ к тому только с одного узла кластера.
- **CephFS** — поддерживает ReadWriteOnce (RWO) и ReadWriteMany (RWX) — одновременный доступ к тому с нескольких узлов кластера.

### Разрешения (caps) для пользователей в Ceph

Для обеспечения корректной работы модуля `csi-ceph` пользователи Ceph должны иметь соответствующие разрешения (caps). Необходимые разрешения зависят от используемого типа хранилища. Ниже приведены примеры правильных конфигураций разрешений для различных сценариев.

#### RBD

Для одного пула с названием `rbd` требуются следующие разрешения:

```ini
[client.name]
        key = key
        caps mgr = "profile rbd pool=rbd"
        caps mon = "profile rbd"
        caps osd = "profile rbd pool=rbd"
```

#### CephFS

Перед настройкой разрешений CephFS убедитесь, что в CephFS создан subvolumegroup `csi` (или другой, указанный в `Custom resources`).

Создать новый subvolumegroup можно командой на узле управления Ceph:

```shell
ceph fs subvolumegroup create <fs_name> <group_name>
```

Например, для создания subvolumegroup `csi` для файловой системы `myfs`:

```shell
ceph fs subvolumegroup create myfs csi
```

Требуемые разрешения для CephFS с названием `myfs`:

```ini
[client.name]
        key = key
        caps mds = "allow rwps fsname=myfs"
        caps mgr = "allow rw"
        caps mon = "allow r fsname=myfs"
        caps osd = "allow rw tag cephfs data=myfs, allow rw tag cephfs metadata=myfs"
```

#### CephFS + RBD

Для пользователя, которому необходим доступ к CephFS `myfs` и RBD пулу `rbd`, объедините разрешения следующим образом:

```ini
[client.name]
        key = key
        caps mds = "allow rwps fsname=myfs"
        caps mgr = "allow rw,profile rbd pool=rbd"
        caps mon = "allow r fsname=myfs,profile rbd"
        caps osd = "allow rw tag cephfs metadata=myfs, allow rw tag cephfs data=myfs,profile rbd pool=rbd"
```
