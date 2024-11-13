---
title: "Реплицируемое хранилище на основе LVM"
permalink: ru/virtualization-platform/documentation/admin/platform-management/storage/sds/lvm-replicated.html
lang: ru
---

Репликация данных между несколькими узлами позволяет обеспечить доступность данных в случае отказа одного из них.
В случае сбоя оборудования или программного обеспечения на одном узле, данные остаются доступными на других узлах.
Репликация также позволяет проводить работы по обслуживанию и обновлению без простоев, поскольку данные остаются доступными благодаря другим репликам.

Чтобы создать реплицируемые блочные StorageClass’ы на базе распределённого реплицируемого блочного устройства 
DRBD (Distributed Replicated Block Device), можно использовать модуль sds-replicated-volume.

После включения модуля sds-replicated-volume в конфигурации Deckhouse ваш кластер будет автоматически настроен на использование бэкенда LINSTOR. 
Останется только создать пулы хранения и StorageClass по инструкции ниже.

## Включение модуля

### Обнаружение компонентов LVM

Перед тем как приступить к настройке возможности создания StorageClass’ов на базе LVM (Logical Volume Manager),
необходимо обнаружить доступные на узлах блочные устройства и группы томов и получить актуальную информацию об их состоянии.
Для этого включите модуль sds-node-configurator:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: sds-node-configurator
spec:
  enabled: true
  version: 1
EOF
```

Дождитесь, когда модуль sds-node-configurator перейдет в состояние Ready.

```shell
d8 k get modules sds-node-configurator -w

# NAME                    WEIGHT   STATE     SOURCE      STAGE   STATUS
# sds-node-configurator   900      Enabled   deckhouse           Ready
```

### Подключение DRBD

Затем, чтобы включить модуль sds-replicated-volume с настройками по умолчанию, выполните команду.

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: sds-replicated-volume
spec:
  enabled: true
  version: 1
EOF
```

Это приведет к тому, что на всех узлах кластера будет установлен модуль ядра DRBD, зарегистрирован CSI драйвер 
и запущены служебные поды компонентов sds-replicated-volume.

Дождитесь, когда модуль sds-replicated-volume перейдет в состояние Ready.

```shell
d8 k get modules sds-replicated-volume -w

# NAME                    WEIGHT   STATE     SOURCE     STAGE   STATUS
# sds-replicated-volume   915      Enabled   Embedded           Ready
```

Чтобы проверить, что в namespace d8-sds-replicated-volume и d8-sds-node-configurator все поды в состоянии Running или Completed
и запущены на всех узлах, где планируется использовать ресурсы DRBD, можно использовать команды:

```shell
d8 k -n d8-sds-replicated-volume get pod -w
d8 k -n d8-sds-node-configurator get pod -w
```

Конфигурация LINSTOR в Deckhouse осуществляется sds-replicated-volume-controller'ом посредством создания 
пользовательских ресурсов: ReplicatedStoragePool и ReplicatedStorageClass. 
Для создания Storage Pool потребуются настроенные на узлах кластера LVM Volume Group и LVM Thin-pool. 
Настройка LVM осуществляется модулем sds-node-configurator.

Следует избегать непосредственной конфигурации бэкенда LINSTOR пользователем, так как это может привести к неожиднному поведению.

## Преднастройка узлов

Процесс преднастройки описан по [ссылке](todo,mc).

Убедитесь, что все созданные ресурсы `LVMVolumeGroup` перешли в состояние `Operational`.

```shell
d8 k get lvg -w

# NAME             THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE       ALLOCATED SIZE   VG   AGE
# vg-on-worker-0   1/1         True                    Ready   worker-0   360484Mi   30064Mi          vg   1h
# vg-on-worker-1   1/1         True                    Ready   worker-1   360484Mi   30064Mi          vg   1h
# vg-on-worker-2   1/1         True                    Ready   worker-2   360484Mi   30064Mi          vg   1h
```

Теперь, когда на узлах созданы нужные группы томов LVM, необходимо объединить их в единое логическое пространство для 
достижения приемуществ реплицированного хранения данных: распределить нагрузку с учётом отказоустойчивости и производительности.
Сделать это можно создав пулы хранения в бэкенде LINSTOR, определив интерфейс взаимодействия через ресурс `ReplicatedStoragePool`:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: ReplicatedStoragePool
metadata:
  name: thick-pool
spec:
  type: LVM
  lvmVolumeGroups:
    - name: vg-1-on-worker-0
    - name: vg-1-on-worker-1
    - name: vg-1-on-worker-2
EOF
```

Созданный выше ReplicatedStoragePool позволит в дальнейшем создавать StorageClass'ы на основе Thick-пула, который обладает
высокой производительностью, сравнимой с производительностью накопителя, но не позволяет использовать snapshot’ы.

Альтернативно можно создать Thin-пул, который позволит использовать snapshot’ы и overprovisioning, но производительность будет ниже.
Пример такого ресурса:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: ReplicatedStoragePool
metadata:
  name: thin-pool
spec:
  type: LVMThin
  lvmVolumeGroups:
    - name: vg-1-on-worker-1
      thinPoolName: thin-pool
    - name: vg-1-on-worker-2
      thinPoolName: thin-pool
EOF
```

Дождитесь, когда созданные ресурсы `ReplicatedStoragePool` перейдут в состояние Completed:

```shell
d8 k get rsp data -w

# NAME         PHASE     AGE
# thick-pool   Created   1h
# thin-pool    Created   1h
```

## Создание StorageClass’а

Создание StorageClass’ов осуществляется через ресурс `ReplicatedStorageClass`, который определяет конфигурацию для желаемого
класса хранения. Ручное создание ресурса `StorageClass` без `ReplicatedStorageClass` может привести к нежелательному поведению.
Пример создания ресурса `ReplicatedStorageClass` на основе thick-пула, `PersistentVolumes` которого будут размещены на группах томов на трех узлах:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: ReplicatedStorageClass
metadata:
  name: replicated-storage-class
spec:
  # Указываем имя ReplicatedStoragePool, созданного ранее.
  storagePool: thick-pool
  # Режим поведения при удалении PVC.
  # Допустимые значения: "Delete", "Retain".
  # [Подробнее...](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#reclaiming)
  reclaimPolicy: Delete
  # Реплики смогут размещаться на любых доступных узлах: не более одной реплики определенного тома на один узел.
  # В кластере нет зон (нет узлов с лейблами topology.kubernetes.io/zone).
  topology: Ignored
  # режим репликации, при котором том остается доступным для чтения и записи, даже если одна из реплик тома становится недоступной. 
  # Данные хранятся в трех экземплярах на разных узлах.
  replication: ConsistencyAndAvailability
EOF
```

Подробности возможности конфигурации ресурса `ReplicatedStorageClass` описаны по [ссылке](todo,mc).

Проверьте, что созданный ресурс `ReplicatedStorageClass` перешел в состояние `Created` и соответствующий `StorageClass` создался:

```shell
d8 k get rsc replicated-storage-class -w

# NAME                       PHASE     AGE
# replicated-storage-class   Created   1h

d8 k get sc replicated-storage-class

# NAME                       PROVISIONER                      RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
# replicated-storage-class   local.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   1h
```

Если `StorageClass` с именем replicated-storage-class появился, значит настройка модуля sds-replicated-volume завершена.
Теперь пользователи могут создавать `PersistentVolume`, указывая `StorageClass` с именем replicated-storage-class.
При указанных выше настройках будет создаваться том с 3мя репликами на разных узлах.

