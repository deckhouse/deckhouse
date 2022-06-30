---
title: "Модуль linstor: примеры конфигурации"
---

## Использование планировщика linstor

Планировщик `linstor` учитывает размещение данных в хранилище и старается размещать Pod в первую очередь на тех узлах, где данные доступны локально. Включается добавлением параметра `schedulerName: linstor` в описание Pod'а приложения.

Пример описания Pod'а, использующего планировщик `linstor`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: busybox
  namespace: default
spec:
  schedulerName: linstor # Использование планировщика linstor
  containers:
  - name: busybox
    image: busybox
    command: ["tail", "-f", "/dev/null"]
    volumeMounts:
    - name: my-first-linstor-volume
      mountPath: /data
    ports:
    - containerPort: 80
  volumes:
  - name: my-first-linstor-volume
    persistentVolumeClaim:
      claimName: "test-volume"
```

## Резервное копирование в S3

> Использование данной возможности требует настроенного мастер-пароля (см. инструкции вначале страницы [конфигурации модуля](configuration.html)).
>
> Резервное копирование с помощью снапшотов поддерживается только для LVMThin-пулов.

Резервное копирование данных реализовано с помощью [снапшотов томов](https://kubernetes.io/docs/concepts/storage/volume-snapshots/). Поддержка работы снапшотов обеспечивается модулем [snapshot-controller](../045-snapshot-controller/), который включается автоматически для поддерживаемых CSI-драйверов в кластерах Kubernetes версий 1.20 и выше.

### Создание резервной копии

Для создания снапшота тома и загрузки его в S3 выполните следующие шаги:

1. Создайте `VolumeSnapshotClass` и `Secret`, содержащий access key и secret key доступа к хранилищу S3.

   > VolumeSnapshotClass — ресурс на уровне кластера. Один и тот же VolumeSnapshotClass можно использовать для создания резервных копий разных PVC из разных пространств имен.

   Пример `VolumeSnapshotClass` и `Secret`:

   ```yaml
   kind: VolumeSnapshotClass
   apiVersion: snapshot.storage.k8s.io/v1
   metadata:
     name: linstor-csi-snapshot-class-s3
   driver: linstor.csi.linbit.com
   deletionPolicy: Retain
   parameters:
     snap.linstor.csi.linbit.com/type: S3
     snap.linstor.csi.linbit.com/remote-name: backup-remote               # Уникальное название backup-подключения в linstor.   
     snap.linstor.csi.linbit.com/allow-incremental: "false"               # Использовать ли инкрементальные копии. 
     snap.linstor.csi.linbit.com/s3-bucket: snapshot-bucket               # Название S3 bucket, для хранения данных.
     snap.linstor.csi.linbit.com/s3-endpoint: s3.us-west-1.amazonaws.com  # S3 endpoint URL.
     snap.linstor.csi.linbit.com/s3-signing-region: us-west-1             # Регион S3. 
     # Использовать virtual hosted–style или path-style S3 URL 
     # https://docs.aws.amazon.com/AmazonS3/latest/userguide/VirtualHosting.html
     snap.linstor.csi.linbit.com/s3-use-path-style: "false"    
     # Ссылка на Secret, содержащий access key и secret key доступа к S3 bucket.
     csi.storage.k8s.io/snapshotter-secret-name: linstor-csi-s3-access
     csi.storage.k8s.io/snapshotter-secret-namespace: storage
   ---
   kind: Secret
   apiVersion: v1
   metadata:
     name: linstor-csi-s3-access
     namespace: storage
   immutable: true
   type: linstor.csi.linbit.com/s3-credentials.v1
   stringData:
     access-key: *!ACCESS_KEY*  # Access key доступа к хранилищу S3.
     secret-key: *!SECRET_KEY*  # Secret key доступа к хранилищу S3.
   ```

1. Выберите (или создайте) `PersistentVolumeClaim`, данные которого нужно копировать.

   Пример `PersistentVolumeClaim`, который будет использоваться в примерах далее:

   ```yaml
   apiVersion: v1
   kind: PersistentVolumeClaim
   metadata:
     name: my-linstor-volume
     namespace: storage
   spec:
     accessModes:
     - ReadWriteOnce
     storageClassName: linstor-thindata-r2   # StorageClass хранилища linstor.
     resources:
       requests:
         storage: 2Gi
   ```

1. Создайте `VolumeSnapshot`.

   Пример `VolumeSnapshot`, использующего `VolumeSnapshotClass` созданный ранее:

   ```yaml
   apiVersion: snapshot.storage.k8s.io/v1
   kind: VolumeSnapshot
   metadata:
     name: my-linstor-snapshot
     namespace: storage
   spec:
     volumeSnapshotClassName: linstor-csi-snapshot-class-s3  # Имя VolumeSnapshotClass, с доступом к хранилищу S3.
     source:
       persistentVolumeClaimName: my-linstor-volume          # Имя PVC, данные с тома которого необходимо копировать. 
   ```

   После создания `VolumeSnapshot` связанного с `PersistentVolumeClaim` относящимся к существующему тому с данными, произойдет создание снапшота в linstor и загрузка его в хранилище S3.

1. Проверьте, статус выполнения резервного копирования.

   Пример:

   ```shell
   kubectl get volumesnapshot my-linstor-snapshot -n storage
   ```

   Если значение READYTOUSE `VolumeSnapshot` не `true`, то посмотрите причину, выполнив следующую команду:  

   ```shell
   kubectl describe volumesnapshot my-linstor-snapshot -n storage
   ```

Посмотреть список и состояние созданных снапшотов в linstor, можно выполнив следующую команду:

```shell
kubectl exec -n d8-linstor deploy/linstor-controller -- linstor snapshot list
```

### Восстановление из резервной копии

Для восстановления данных в том же пространстве имен, в котором был создан VolumeSnapshot, достаточно создать PVC со ссылкой на необходимый VolumeSnapshot.

Пример PVC для восстановления из VolumeSnapshot `example-backup-from-s3` в том же пространстве имен:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: restored-data
  namespace: storage
spec:
  storageClassName: "linstor-thindata-r1" # Имя StorageClass тома для восстановления данных.  
  dataSource:
    name: example-backup-from-s3          # Имя созданного ранее VolumeSnapshot.
    kind: VolumeSnapshot
    apiGroup: snapshot.storage.k8s.io
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 2Gi
```

Для восстановления данных из хранилища S3 в другом пространстве имен или кластере Kubernetes, выполните следующие шаги:

1. Создайте `VolumeSnapshotClass` и `Secret`, содержащий access key и secret key доступа к хранилищу S3, если они не были созданы ранее (например, если вы восстанавливаете данные в новом кластере).

   Пример `VolumeSnapshotClass` и `Secret`:

   ```yaml
   kind: VolumeSnapshotClass
   apiVersion: snapshot.storage.k8s.io/v1
   metadata:
     name: linstor-csi-snapshot-class-s3
   driver: linstor.csi.linbit.com
   deletionPolicy: Retain
   parameters:
     snap.linstor.csi.linbit.com/type: S3
     snap.linstor.csi.linbit.com/remote-name: backup-remote               # Уникальное название backup-подключения в linstor.   
     snap.linstor.csi.linbit.com/allow-incremental: "false"               # Использовать ли инкрементальные копии. 
     snap.linstor.csi.linbit.com/s3-bucket: snapshot-bucket               # Название S3 bucket, для хранения данных.
     snap.linstor.csi.linbit.com/s3-endpoint: s3.us-west-1.amazonaws.com  # S3 endpoint URL.
     snap.linstor.csi.linbit.com/s3-signing-region: us-west-1             # Регион S3. 
     # Использовать virtual hosted–style или path-style S3 URL 
     # https://docs.aws.amazon.com/AmazonS3/latest/userguide/VirtualHosting.html
     snap.linstor.csi.linbit.com/s3-use-path-style: "false"    
     # Ссылка на Secret, содержащий access key и secret key доступа к S3 bucket.
     csi.storage.k8s.io/snapshotter-secret-name: linstor-csi-s3-access
     csi.storage.k8s.io/snapshotter-secret-namespace: storage
   ---
   kind: Secret
   apiVersion: v1
   metadata:
     name: linstor-csi-s3-access
     namespace: storage
   immutable: true
   type: linstor.csi.linbit.com/s3-credentials.v1
   stringData:
     access-key: *!ACCESS_KEY*  # Access key доступа к хранилищу S3.
     secret-key: *!SECRET_KEY*  # Secret key доступа к хранилищу S3.
   ```

1. Получите id снапшота для восстановления одним из следующих способов:

   1. Получите список снапшотов в кластере linstor, и выберите нужный (колонка `SnapshotName`):

      ```shell
      kubectl exec -n d8-linstor deploy/linstor-controller -- linstor backup list <backup-remote-name>
      ```

      , где `<backup-remote-name>` — название backup-подключения, использованное в `VolumeSnapshotClass`.

   1. Получите id-снапшота из имени объекта в S3-бакете через UI-интерфейс или CLI-утилиты S3-сервиса.

1. Создайте `VolumeSnapshotContent`, указывающий на конкретный id снапшота.

   Пример:

   ```yaml
   apiVersion: snapshot.storage.k8s.io/v1
   kind: VolumeSnapshotContent
   metadata:
     name: restored-snap-content-from-s3
   spec:
     deletionPolicy: Delete
     driver: linstor.csi.linbit.com
     source:
       snapshotHandle: *!snapshot_id*                        # ID снапшота.  
     volumeSnapshotClassName: linstor-csi-snapshot-class-s3  # Имя VolumeSnapshotClass, с доступом к хранилищу S3.
     volumeSnapshotRef:
       apiVersion: snapshot.storage.k8s.io/v1
       kind: VolumeSnapshot
       name: example-backup-from-s3                          # Имя VolumeSnapshot, который будет создан далее.
       namespace: storage
   ```

1. Создайте `VolumeSnapshot`, указывающий на созданный `VolumeSnapshotContent`.

   Пример:

   ```yaml
   apiVersion: snapshot.storage.k8s.io/v1
   kind: VolumeSnapshot
   metadata:
     name: example-backup-from-s3
     namespace: storage
   spec:
     source:
       volumeSnapshotContentName: restored-snap-content-from-s3 # Имя VolumeSnapshotContent, созданного ранее.
     volumeSnapshotClassName: linstor-csi-snapshot-class-s3     # Имя VolumeSnapshotClass, с доступом к хранилищу S3.
   ```

1. Создайте `PersistentVolumeClaim`.

   Пример:

   ```yaml
   apiVersion: v1
   kind: PersistentVolumeClaim
   metadata:
     name: restored-data
     namespace: storage
   spec:
     storageClassName: "linstor-thindata-r1" # Имя StorageClass тома для восстановления данных.  
     dataSource:
       name: example-backup-from-s3          # Имя созданного ранее VolumeSnapshot.
       kind: VolumeSnapshot
       apiGroup: snapshot.storage.k8s.io
     accessModes:
       - ReadWriteOnce
     resources:
       requests:
         storage: 2Gi
   ```

Используйте созданный `PersistentVolumeClaim` для доступа к копии восстановленных данных.
