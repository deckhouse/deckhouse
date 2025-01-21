---
title: "Реплицируемое хранилище"
permalink: ru/virtualization-platform/documentation/admin/platform-management/storage/sds/lvm-replicated.html
lang: ru
---

Репликация данных между несколькими узлами позволяет обеспечить отказоустойчивость и доступность данных даже при сбоях в оборудовании или программном обеспечении одного из узлов. Это гарантирует, что данные сохранятся на других узлах, а доступ к ним будет непрерывным. Такая модель необходима для критически важных данных, а также для распределенных инфраструктур, где требуется высокая доступность и минимизация потерь данных в случае сбоев.

Чтобы создать реплицируемые блочные объекты StorageClass на базе распределённого реплицируемого блочного устройства DRBD (Distributed Replicated Block Device), можно задействовать модуль sds-replicated-volume, который использует [LINSTOR](https://linbit.com/linstor/) в качестве своего бэкенда.

## Включение модуля

### Обнаружение компонентов LVM

Перед тем как приступить к созданию объектов StorageClass на базе LVM (Logical Volume Manager), необходимо обнаружить доступные на узлах блочные устройства и группы томов и получить актуальную информацию об их состоянии.

Для этого включите модуль `sds-node-configurator`:

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

Дождитесь, когда модуль `sds-node-configurator` перейдет в состояние `Ready`.
Проверить состояние можно, выполнив следующую команду:

```shell
d8 k get modules sds-node-configurator -w
```

В результате будет выведена информация о модуле `sds-node-configurator`:

```console
NAME                    WEIGHT   STATE     SOURCE      STAGE   STATUS
sds-node-configurator   900      Enabled   deckhouse           Ready
```

### Подключение DRBD

Чтобы включить модуль `sds-replicated-volume` с настройками по умолчанию, выполните команду:

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

Это приведет к тому, что на всех узлах кластера будет установлен модуль ядра DRBD, зарегистрирован CSI драйвер и запущены служебные поды компонентов `sds-replicated-volume`.

Дождитесь, когда модуль `sds-replicated-volume` перейдет в состояние `Ready`.
Проверить состояние можно, выполнив следующую команду:

```shell
d8 k get modules sds-replicated-volume -w
```

В результате будет выведена информация о модуле `sds-replicated-volume`:

```console
NAME                    WEIGHT   STATE     SOURCE     STAGE   STATUS
sds-replicated-volume   915      Enabled   Embedded           Ready
```

Чтобы проверить, что в пространстве имен `d8-sds-replicated-volume` и `d8-sds-node-configurator` все поды в состоянии `Running` или `Completed` и запущены на всех узлах, где планируется использовать ресурсы DRBD, можно использовать команды:

```shell
d8 k -n d8-sds-replicated-volume get pod -w
d8 k -n d8-sds-node-configurator get pod -w
```

Следует избегать непосредственной конфигурации бэкенда LINSTOR пользователем, так как это может привести к ошибкам.

## Преднастройка узлов

### Создание групп томов LVM

Перед тем как приступить к настройке создания объектов StorageClass, необходимо объединить доступные на узлах блочные устройства в группы томов LVM. В дальнейшем группы томов будут использоваться для размещения PersistentVolume. Чтобы получить доступные блочные устройства, можно использовать ресурс `BlockDevices`, который отражает их актуальное состояние:

```shell
d8 k get bd
```

В результате будет выведен список доступных блочных устройств:

```console
NAME                                           NODE       CONSUMABLE   SIZE           PATH
dev-ef4fb06b63d2c05fb6ee83008b55e486aa1161aa   worker-0   false        976762584Ki    /dev/nvme1n1
dev-0cfc0d07f353598e329d34f3821bed992c1ffbcd   worker-0   false        894006140416   /dev/nvme0n1p6
dev-7e4df1ddf2a1b05a79f9481cdf56d29891a9f9d0   worker-1   false        976762584Ki    /dev/nvme1n1
dev-b103062f879a2349a9c5f054e0366594568de68d   worker-1   false        894006140416   /dev/nvme0n1p6
dev-53d904f18b912187ac82de29af06a34d9ae23199   worker-2   false        976762584Ki    /dev/nvme1n1
dev-6c5abbd549100834c6b1668c8f89fb97872ee2b1   worker-2   false        894006140416   /dev/nvme0n1p6
```

В примере выполнения команды выше, в наличии имеется шесть блочных устройств, расположенных на трёх узлах.

Чтобы объединить блочные устройства на одном узле, необходимо создать группу томов LVM с помощью ресурса `LVMVolumeGroup`.
Для создания ресурса `LVMVolumeGroup` на узле worker-0 примените следующий ресурс, предварительно заменив имена узла и блочных устройств на свои:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: LVMVolumeGroup
metadata:
  name: "vg-on-worker-0"
spec:
  type: Local
  local:
    # Замените на имя своего узла, для которого создаете группу томов. 
    nodeName: "worker-0"
  blockDeviceSelector:
    matchExpressions:
      - key: kubernetes.io/metadata.name
        operator: In
        values:
          # Замените на имена своих блочных устройств узла, для которого создаете группу томов. 
          - dev-ef4fb06b63d2c05fb6ee83008b55e486aa1161aa
          - dev-0cfc0d07f353598e329d34f3821bed992c1ffbcd
  # Имя группы томов LVM, которая будет создана из указанных выше блочных устройств на выбранном узле.
  actualVGNameOnTheNode: "vg"
  # Раскомментируйте, если важно иметь возможность создавать Thin-пулы, детали будут раскрыты далее.
  # thinPools:
  #   - name: thin-pool-0
  #     size: 70%
EOF
```

Подробности о возможностях конфигурации ресурса `LVMVolumeGroup` описаны в разделе [«Справка»](../../../../../reference/cr/lvmvolumegroup.html).

Дождитесь, когда созданный ресурс `LVMVolumeGroup` перейдет в состояние `Ready`.
Чтобы проверить состояние ресурса, выполните следующую команду:

```shell
d8 k get lvg vg-on-worker-0 -w
```

В результате будет выведена информация о состоянии ресурса:

```console
NAME             THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE       ALLOCATED SIZE   VG   AGE
vg-on-worker-0   1/1         True                    Ready   worker-0   360484Mi   30064Mi          vg   1h
```

Если ресурс перешел в состояние `Ready`, то это значит, что на узле worker-0
из блочных устройств `/dev/nvme1n1` и `/dev/nvme0n1p6` была создана группа томов LVM с именем `vg`.

Далее необходимо повторить создание ресурсов `LVMVolumeGroup` для оставшихся узлов (worker-1 и worker-2), изменив в примере выше имя ресурса `LVMVolumeGroup`, имя узла и имена блочных устройств, соответствующих узлу.

Убедитесь, что группы томов LVM созданы на всех узлах, где планируется их использовать, выполнив следующую команду:

```shell
d8 k get lvg -w
```

В результате будет выведен список созданных групп томов:

```console
NAME             THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE       ALLOCATED SIZE   VG   AGE
vg-on-worker-0   0/0         True                    Ready   worker-0   360484Mi   30064Mi          vg   1h
vg-on-worker-1   0/0         True                    Ready   worker-1   360484Mi   30064Mi          vg   1h
vg-on-worker-2   0/0         True                    Ready   worker-2   360484Mi   30064Mi          vg   1h
```

### Создание реплицированных Thick-пулов

Теперь, когда на узлах созданы нужные группы томов LVM, необходимо объединить их в единое логическое пространство. Это можно сделать, объединив их в реплицированные пулы хранения в бэкенде LINSTOR через интерфейс в виде ресурса `ReplicatedStoragePool`.

Пулы хранения могут быть двух типов: LVM (Thick) и LVMThin (Thin). Thick-пул обладает высокой производительностью, сравнимой с производительностью накопителя, но не позволяет использовать снапшоты, в то время как Thin-пул позволит использовать снапшоты, но производительность будет ниже.

Пример создания реплицированного Thick-пула:

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

Подробности о возможностях конфигурации ресурса `ReplicatedStoragePool` описаны в разделе [«Справка»](../../../../../reference/cr/replicatedstoragepool.html).

Дождитесь, когда созданный ресурс `ReplicatedStoragePool` перейдет в состояние `Completed`.
Чтобы проверить состояние ресурса, выполните следующую команду:

```shell
d8 k get rsp data -w
```

В результате будет выведена информация о состоянии созданного ресурса:

```console
NAME         PHASE       TYPE   AGE
thick-pool   Completed   LVM    87d
```

### Создание реплицированных Thin-пулов

Созданные ранее `LVMVolumeGroup` подходят для создания Thick-пулов. Если вам важно иметь возможность создавать реплицированные Thin-пулы, обновите конфигурацию ресурсов `LVMVolumeGroup`, добавив определение для Thin-пула:

```yaml
d8 k patch lvg vg-on-worker-0 --type='json' -p='[
  {
    "op": "add",
    "path": "/spec/thinPools",
    "value": [
      {
        "name": "thin-pool-0",
        "size": "70%"
      }
    ]
  }
]'
```

В обновленной версии `LVMVolumeGroup` 70% доступного пространства будет использовано для создания Thin-пулов. Оставшиеся 30% могут быть использованы для Thick-пулов.

Повторите добавление Thin-пулов для оставшихся узлов (worker-1 и worker-2).
Пример создания реплицированного Thin-пула:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: ReplicatedStoragePool
metadata:
  name: thin-pool
spec:
  type: LVMThin
  lvmVolumeGroups:
    - name: vg-1-on-worker-0
      thinPoolName: thin-pool-0
    - name: vg-1-on-worker-1
      thinPoolName: thin-pool-0
    - name: vg-1-on-worker-2
      thinPoolName: thin-pool-0
EOF
```

Подробности о возможностях конфигурации ресурса `ReplicatedStoragePool` описаны в разделе [«Справка»](../../../../../reference/cr/replicatedstoragepool.html).

Дождитесь, когда созданный ресурс `ReplicatedStoragePool` перейдет в состояние `Completed`.
Чтобы проверить состояние ресурса, выполните следующую команду:

```shell
d8 k get rsp data -w
```

В результате будет выведена информация о состоянии созданного ресурса:

```console
NAME        PHASE       TYPE      AGE
thin-pool   Completed   LVMThin   87d
```

## Создание объектов StorageClass

Создание объектов StorageClass осуществляется через ресурс `ReplicatedStorageClass`, который определяет конфигурацию для желаемого класса хранения. Ручное создание ресурса StorageClass без `ReplicatedStorageClass` может привести к нежелательному поведению.
Пример создания ресурса `ReplicatedStorageClass` на основе Thick-пула, `PersistentVolumes` которого будут размещены на группах томов на трех узлах:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: ReplicatedStorageClass
metadata:
  name: replicated-storage-class
spec:
  # Указываем имя одного из пулов хранения, созданных ранее.
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

Подробности о возможностях конфигурации ресурса `ReplicatedStorageClass` описаны в разделе [«Справка»](../../../../../reference/cr/replicatedstorageclass.html).

Проверьте, что созданный ресурс `ReplicatedStorageClass` перешел в состояние `Created`, выполнив следующую команду:

```shell
d8 k get rsc replicated-storage-class -w
```

В результате будет выведена информация о созданном `ReplicatedStorageClass`:

```console
NAME                       PHASE     AGE
replicated-storage-class   Created   1h
```

 Убедитесь, что был создан соответствующий StorageClass, выполнив следующую команду:

```shell
d8 k get sc replicated-storage-class
```

В результате будет выведена информация о созданном StorageClass:

```console
NAME                       PROVISIONER                      RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
replicated-storage-class   local.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   1h
```

Если StorageClass с именем `replicated-storage-class` появился, значит настройка модуля sds-replicated-volume завершена.
Теперь пользователи могут создавать `PersistentVolume`, указывая StorageClass с именем `replicated-storage-class`.
При указанных выше настройках будет создаваться том с тремя репликами на разных узлах.
