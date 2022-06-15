---
title: "Модуль linstor: расширенная конфигурация"
---

[Упрощенное руководство](configuration.html#конфигурация-хранилища-linstor) содержит шаги, в результате выполнения которых автоматически создаются storage-пулы и StorageClass'ы, при появлении на узле LVM-группы томов или LVMThin-пула с тегом `linstor-<имя_пула>`. Далее рассматривается шаги по ручному созданию storage-пулов и StorageClass'ов.

Для выполнения дальнейших действий потребуется CLI-утилита `linstor`. Используйте один из следующих вариантов запуска утилиты `linstor`:
- Установите плагин [kubectl-linstor](https://github.com/piraeusdatastore/kubectl-linstor).
- Добавьте alias в BASH для запуска утилиты `linstor` из Pod'а контроллера linstor:

  ```shell
  alias linstor='kubectl exec -n d8-linstor deploy/linstor-controller -- linstor'
  ```

После включения модуля `linstor` кластер и его узлы настраиваются на использование LINSTOR автоматически. Для того чтобы начать использовать хранилище, необходимо:

- [Создать пулы хранения](#создание-пулов-хранения)
- [Создать StorageClass](#создание-storageclass)

## Создание пулов хранения

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

## Создание StorageClass

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
allowVolumeExpansion: true
provisioner: linstor.csi.linbit.com
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer
```
