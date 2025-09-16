---
title: "NFS-хранилище"
permalink: ru/virtualization-platform/documentation/admin/platform-management/storage/external/nfs.html
lang: ru
---

Для управления томами на основе протокола NFS (Network File System) можно использовать модуль `csi-nfs`, позволяющий создавать StorageClass через создание пользовательских ресурсов `NFSStorageClass`.

## Включение модуля

Чтобы включить модуль `csi-nfs`, выполните команду:

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

Дождитесь, когда модуль `csi-nfs` перейдет в состояние `Ready`.
Проверить состояние можно, выполнив следующую команду:

```shell
d8 k get module csi-nfs -w
```

В результате будет выведена информация о модуле `csi-nfs`:

```console
NAME      STAGE   SOURCE   PHASE       ENABLED   READY
csi-nfs                    Available   True      True
```

## Создание StorageClass

Для создания StorageClass необходимо использовать ресурс `NFSStorageClass`.
Ручное создание ресурса StorageClass без `NFSStorageClass` может привести к ошибкам.

Пример команды для создания класса хранения на базе NFS:

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

Проверьте, что созданный ресурс `NFSStorageClass` перешел в состояние `Created`, выполнив следующую команду:

```shell
d8 k get NFSStorageClass nfs-storage-class -w
```

В результате будет выведена информация о созданном ресурсе `NFSStorageClass`:

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

Если StorageClass с именем `nfs-storage-class` появился, значит настройка модуля csi-nfs завершена.
Теперь пользователи могут создавать PersistentVolume, указывая StorageClass с именем `nfs-storage-class`.

Для каждого ресурса PersistentVolume будет создаваться каталог `<директория из share>/<имя PersistentVolume>`.

## Проверка работоспособности модуля

Для того, чтобы проверить работоспособность модуля csi-nfs, необходимо проверить состояние подов в пространстве имен d8-csi-nfs.
Все поды должны быть в состоянии `Running` или `Completed`, поды csi-nfs должны быть запущены на всех узлах.

Проверить работоспособность модуля можно с помощью следующей команды:

```shell
d8 k -n d8-csi-nfs get pod -owide -w
```

В результате будет выведен список всех подов в пространстве имен d8-csi-nfs:

```console
NAME                             READY   STATUS    RESTARTS   AGE   IP             NODE       NOMINATED NODE   READINESS GATES
controller-547979bdc7-5frcl      1/1     Running   0          1h    10.111.2.84    master     <none>           <none>
csi-controller-5c6bd5c85-wzwmk   6/6     Running   0          1h    172.18.18.50   master     <none>           <none>
webhooks-7b5bf9dbdb-m5wxb        1/1     Running   0          1h    10.111.0.16    master     <none>           <none>
csi-nfs-8mpcd                    2/2     Running   0          1h    172.18.18.50   master     <none>           <none>
csi-nfs-n6sks                    2/2     Running   0          1h    172.18.18.51   worker-1   <none>           <none>
csi-nfs-6nqq8                    2/2     Running   0          1h    172.18.18.52   worker-2   <none>           <none>
```
