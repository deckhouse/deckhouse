---
title: "Модуль linstor: расширенная конфигурация"
---

[Упрощенное руководство](configuration.html#конфигурация-хранилища-linstor) содержит шаги, в результате выполнения которых автоматически создаются пулы хранения (storage-пулы) и StorageClass'ы, при появлении на узле LVM-группы томов или LVMThin-пула с тегом `linstor-<имя_пула>`. Далее рассматривается шаги по ручному созданию пулов хранения и StorageClass'ов.

Для выполнения дальнейших действий потребуется CLI-утилита `linstor`. Используйте один из следующих вариантов запуска утилиты `linstor`:
- Установите плагин [kubectl-linstor](https://github.com/piraeusdatastore/kubectl-linstor).
- Добавьте alias в BASH для запуска утилиты `linstor` из Pod'а контроллера linstor:

  ```shell
  alias linstor='kubectl exec -n d8-linstor deploy/linstor-controller -- linstor'
  ```

> Большинство пунктов на этой странице позаимствованы из [официальной документации LINSTOR](https://linbit.com/drbd-user-guide/linstor-guide-1_0-en/).  
> Несмотря на то, что здесь мы постарались собрать наиболее распространённые вопросы, не стесняйтесь обращаться к первоисточнику.

## Ручная конфигурация

После включения модуля `linstor` кластер и его узлы настраиваются на использование LINSTOR автоматически. Для того чтобы начать использовать хранилище, необходимо:

- [Создать пулы хранения](#создание-пулов-хранения)
- [Создать StorageClass](#создание-storageclass)

### Создание пулов хранения

1. Отобразите список всех узлов и блочных устройств для хранения.
   - Отобразите список всех узлов:

     ```shell
     linstor node list
     ```

     Пример вывода:
  
     ```text
     +----------------------------------------------------------------------------------------+
     | Node                                | NodeType   | Addresses                  | State  |
     |========================================================================================|
     | node01                              | SATELLITE  | 192.168.199.114:3367 (SSL) | Online |
     | node02                              | SATELLITE  | 192.168.199.60:3367 (SSL)  | Online |
     | node03                              | SATELLITE  | 192.168.199.74:3367 (SSL)  | Online |
     | linstor-controller-85455fcd76-2qhmq | CONTROLLER | 10.111.0.78:3367 (SSL)     | Online |
     +----------------------------------------------------------------------------------------+
     ```

   - Отобразите список всех доступных блочных устройств для хранения:

     ```shell
     linstor physical-storage list
     ```
  
     Пример вывода:
  
     ```text
     +----------------------------------------------------------------+
     | Size          | Rotational | Nodes                             |
     |================================================================|
     | 1920383410176 | False      | node01[/dev/nvme1n1,/dev/nvme0n1] |
     | 1920383410176 | False      | node02[/dev/nvme1n1,/dev/nvme0n1] |
     | 1920383410176 | False      | node03[/dev/nvme1n1,/dev/nvme0n1] |
     +----------------------------------------------------------------+
     ```

     > **Обратите внимание:** отображаются только пустые устройства, без какой-либо разметки.
     > Тем не менее, создание пулов хранения из разделов и других блочных устройств также поддерживается.
     >
     > Вы также можете [добавить](faq.html#как-добавить-существующий-lvm-или-lvmthin-пул) уже существующий пул LVM или LVMthin в кластер.

1. Создайте пулы LVM или LVMThin.

   На необходимых узлах хранилища создайте несколько пулов из устройств, полученных на предыдущем шаге. Их названия должны быть одинаковыми, в случае если вы хотите иметь один storageClass.

   - Пример команды создания **LVM-пула** хранения из двух устройств на одном из узлов:

     ```shell
     linstor physical-storage create-device-pool lvm node01 /dev/nvme0n1 /dev/nvme1n1 --pool-name linstor_data --storage-pool lvm
     ```

     , где:
     - `--pool-name` — имя VG/LV создаваемом на узле.
     - `--storage-pool` — то, как будет называться пул хранения в LINSTOR.

   - Пример команды создания **ThinLVM-пула** хранения из двух устройств на одном из узлов:

     ```shell
     linstor physical-storage create-device-pool lvmthin node01 /dev/nvme0n1 /dev/nvme1n1 --pool-name data --storage-pool lvmthin
     ```

     , где:
     - `--pool-name` — имя VG/LV создаваемом на узле.
     - `--storage-pool` — то, как будет называться пул хранения в LINSTOR.

1. Проверьте создание пулов хранения.

   Как только пулы хранения созданы, можете увидеть их выполнив следующую команду:

   ```shell
   linstor storage-pool list
   ```

   Пример вывода:

   ```text
   +---------------------------------------------------------------------------------------------------------------------------------+
   | StoragePool          | Node   | Driver   | PoolName          | FreeCapacity | TotalCapacity | CanSnapshots | State | SharedName |
   |=================================================================================================================================|
   | DfltDisklessStorPool | node01 | DISKLESS |                   |              |               | False        | Ok    |            |
   | DfltDisklessStorPool | node02 | DISKLESS |                   |              |               | False        | Ok    |            |
   | DfltDisklessStorPool | node03 | DISKLESS |                   |              |               | False        | Ok    |            |
   | lvmthin              | node01 | LVM_THIN | linstor_data/data |     3.49 TiB |      3.49 TiB | True         | Ok    |            |
   | lvmthin              | node02 | LVM_THIN | linstor_data/data |     3.49 TiB |      3.49 TiB | True         | Ok    |            |
   | lvmthin              | node03 | LVM_THIN | linstor_data/data |     3.49 TiB |      3.49 TiB | True         | Ok    |            |
   +---------------------------------------------------------------------------------------------------------------------------------+
   ```

### Создание StorageClass

Создайте StorageClass, где:
- в `parameters."linstor.csi.linbit.com/placementCount"` укажите необходимое количество реплик;
- в `parameters."linstor.csi.linbit.com/storagePool"` укажите имя пула хранения, в котором будут создаваться реплики.

Пример StorageClass:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: linstor-r2
parameters:
  linstor.csi.linbit.com/storagePool: lvmthin
  linstor.csi.linbit.com/placementCount: "2"
  property.linstor.csi.linbit.com/DrbdOptions/Net/rr-conflict: retry-connect
  property.linstor.csi.linbit.com/DrbdOptions/Resource/on-no-data-accessible: suspend-io
  property.linstor.csi.linbit.com/DrbdOptions/Resource/on-suspended-primary-outdated: force-secondary
  property.linstor.csi.linbit.com/DrbdOptions/auto-quorum: suspend-io
allowVolumeExpansion: true
provisioner: linstor.csi.linbit.com
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer
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
linstor snapshot list
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
      linstor backup list <backup-remote-name>
      ```

      , где `<backup-remote-name>` — название backup-подключения, использованное в `VolumeSnapshotClass`.

   1. Получите id-снапшота из имени объекта в S3-бакете через UI-интерфейс или CLI-утилиты S3-сервиса.

1. Создайте `VolumeSnapshotContent`, указывающий на конкретный id снапшота.

   > VolumeSnapshotContent — ресурс на уровне кластера. Каждый VolumeSnapshotClass может быть связан только с одним VolumeSnapshot. Поэтому удостоверьтесь в уникальности его имени.

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

### Запланированное создание резервных копий

LINSTOR поддерживает автоматическое создание резервных копий по расписанию.  
Однако на данный момент эта возмоность доступна только через LINSTOR CLI.

Для этого вам нужно сначала создать S3 remote:

```bash
linstor remote create s3 myRemote s3.us-west-2.amazonaws.com \
  my-bucket us-west-2 admin password [--use-path-style]
```

После этого создать расписание и включить его для вашего remote.  
Для этого пожалуйста обратитесь к [официальной документации LINSTOR](https://linbit.com/drbd-user-guide/linstor-guide-1_0-en/#s-linstor-scheduled-backup-shipping)
