---
title: "Хранилище данных NFS"
permalink: ru/storage/admin/external/nfs.html
lang: ru
---

Deckhouse поддерживает работу с NFS (Network File System), обеспечивая возможность подключения и управления сетевыми файловыми хранилищами в Kubernetes. Это позволяет организовать централизованное хранение данных и совместное использование файлов между контейнерами.

На этой странице представлены инструкции по подключению NFS-хранилища в Deckhouse, настройке соединения, созданию StorageClass, а также проверке работоспособности системы.

## Включение модуля

Для управления томами на основе протокола NFS (Network File System) используется модуль `csi-nfs`, позволяющий создавать StorageClass через создание пользовательских ресурсов [NFSStorageClass](../../../reference/cr/nfsstorageclass/). Чтобы включить модуль выполните команду:

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

Дождитесь, когда модуль `csi-nfs` перейдет в состояние `Ready`. Проверить состояние можно, выполнив следующую команду:

```shell
d8 k get module csi-nfs -w
```

В результате будет выведена информация о модуле `csi-nfs`:

```console
NAME      WEIGHT   STATE     SOURCE     STAGE   STATUS
csi-nfs   910      Enabled   Embedded           Ready
```

## Создание StorageClass

Для создания StorageClass необходимо использовать ресурс [NFSStorageClass](../../../reference/cr/nfsstorageclass/). Ручное создание ресурса StorageClass без [NFSStorageClass](../../../reference/cr/nfsstorageclass/) может привести к ошибкам. Пример команды для создания класса хранения на базе NFS:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: NFSStorageClass
metadata:
  name: nfs-storage-class
spec:
  connection:
    # Адрес NFS сервера.
    host: 10.223.187.3
    # Путь к точке монтирования на NFS сервере.
    share: /
    # Версия NFS сервера.
    nfsVersion: "4.1"
  # Режим поведения при удалении PVC.
  # Допустимые значения:
  # - Delete (при удалении PVC будет удален PV и данные на NFS-сервере);
  # - Retain (при удалении PVC не будут удалены PV и данные на NFS-сервере, потребуют ручного удаления пользователем).
  # [Подробнее...](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#reclaiming)
  reclaimPolicy: Delete
  # Режим создания тома.
  # Допустимые значения: "Immediate", "WaitForFirstConsumer". 
  # [Подробнее...](https://kubernetes.io/docs/concepts/storage/storage-classes/#volume-binding-mode)
  volumeBindingMode: WaitForFirstConsumer
EOF
```

Проверьте, что созданный ресурс [NFSStorageClass](../../../reference/cr/nfsstorageclass/) перешел в состояние `Created`, выполнив следующую команду:

```shell
d8 k get NFSStorageClass nfs-storage-class -w
```

В результате будет выведена информация о созданном ресурсе [NFSStorageClass](../../../reference/cr/nfsstorageclass/):

```console
NAME                PHASE     AGE
nfs-storage-class   Created   1h
```

Убедитесь, что был создан соответствующий StorageClass, выполнив следующую команду:

```shell
d8 k get sc nfs-storage-class
```

В результате будет выведена информация о созданном StorageClass:

```console
NAME                PROVISIONER      RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
nfs-storage-class   nfs.csi.k8s.io   Delete          WaitForFirstConsumer   true                   1h
```

Если StorageClass с именем `nfs-storage-class` появился, значит настройка модуля `csi-nfs` завершена. Теперь пользователи могут создавать PersistentVolume, указывая StorageClass с именем `nfs-storage-class`. Для каждого ресурса PersistentVolume будет создаваться каталог `<директория из share>/<имя PersistentVolume>`.

### Проверка работоспособности модуля

Для проверки работоспособности модуля убедитесь, что все поды в пространстве имён `d8-csi-nfs`находятся в статусе `Running` или `Completed` и запущены на каждом узле кластера:

```shell
d8 k -n d8-csi-nfs get pod -owide -w
```

В результате будет выведен список всех подов в пространстве имен `d8-csi-nfs`:

```console
NAME                             READY   STATUS    RESTARTS   AGE   IP             NODE       NOMINATED NODE   READINESS GATES
controller-547979bdc7-5frcl      1/1     Running   0          1h    10.111.2.84    master     <none>           <none>
csi-controller-5c6bd5c85-wzwmk   6/6     Running   0          1h    172.18.18.50   master     <none>           <none>
webhooks-7b5bf9dbdb-m5wxb        1/1     Running   0          1h    10.111.0.16    master     <none>           <none>
csi-nfs-8mpcd                    2/2     Running   0          1h    172.18.18.50   master     <none>           <none>
csi-nfs-n6sks                    2/2     Running   0          1h    172.18.18.51   worker-1   <none>           <none>
csi-nfs-6nqq8                    2/2     Running   0          1h    172.18.18.52   worker-2   <none>           <none>
```

## Изменение параметров NFS-сервера для уже созданных PV

Изменить параметры подключения к NFS-серверу для уже созданных PersistentVolume невозможно. Эти параметры сохраняются напрямую в манифесте PV и не подлежат изменению. Также изменение StorageClass не приведёт к обновлению настроек подключения в уже существующих PV.

## Создание снимков томов

В Deckhouse снимки создаются путём архивирования папки тома. Архив сохраняется в корне папки на NFS-сервере, указанной в параметре `spec.connection.share`. Для создания снимков:

1. Включите модуль `snapshot-controller`:

   ```yaml
   d8 k apply -f -<<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: snapshot-controller
   spec:
     enabled: true
     version: 1
   EOF
   ```

1. Создайте снимок тома, указав необходимые параметры:

   ```yaml
   d8 k apply -f -<<EOF
   apiVersion: snapshot.storage.k8s.io/v1
   kind: VolumeSnapshot
   metadata:
     name: my-snapshot
     namespace: <пространство имён, в котором находится PVC>
   spec:
     volumeSnapshotClassName: csi-nfs-snapshot-class
     source:
       persistentVolumeClaimName: <пространство имён, для которого необходимо создать снимок>
   EOF
   ```

1. Проверьте состояние созданного снимка командой:

   ```shell
   d8 k get volumesnapshot
   ```

Команда выведет список всех снимков и их текущее состояние.

## Почему не удаляются PV при включённой поддержке RPC-with-TLS

Если ресурс [NFSStorageClass](../../../reference/cr/nfsstorageclass/) настроен с поддержкой RPC-with-TLS, возможно, не удастся удалить созданные PV. Это может произойти, если удалён секрет, содержащий параметры монтирования (например, после удаления [NFSStorageClass](../../../reference/cr/nfsstorageclass/)). В результате контроллер не сможет смонтировать папку на NFS-сервере для удаления каталога `<имя PV>`.

## Как добавить несколько CA в параметр tlsParameters.ca в ModuleConfig

Для добавления нескольких сертификатов CA в параметр `tlsParameters.ca` необходимо объединить их в один файл и закодировать в Base64:

- Для двух CA:

  ```shell
  cat CA1.crt CA2.crt | base64 -w0
  ```

- Для трёх CA:

  ```shell
  cat CA1.crt CA2.crt CA3.crt | base64 -w0
  ```

- И так далее.
