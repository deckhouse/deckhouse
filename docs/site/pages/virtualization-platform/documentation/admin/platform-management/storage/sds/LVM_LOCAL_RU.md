---
title: "Локальное хранилище"
permalink: ru/virtualization-platform/documentation/admin/platform-management/storage/sds/lvm-local.html
lang: ru
---

Использование локального хранилища помогает избежать сетевых задержек, что повышает производительность по сравнению с удалёнными хранилищами, которые требуют подключения через сеть. Этот подход идеально подходит для тестовых сред и EDGE-кластеров.

Чтобы создать локальные блочные объекты StorageClass, можно использовать модуль `sds-local-volume`.  

## Включение модуля

Настройка локального блочного хранилища происходит на основе логического менеджера томов LVM (Logical Volume Manager).
Управление LVM осуществляется модулем sds-node-configurator, который необходимо включить перед активацией модуля sds-local-volume.

Чтобы включить модуль, примените ресурс `ModuleConfig`:

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
NAME                    STAGE   SOURCE      PHASE       ENABLED   READY
sds-node-configurator           deckhouse   Available   True      True
```

Затем, чтобы включить модуль `sds-local-volume` с настройками по умолчанию, выполните команду:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: sds-local-volume
spec:
  enabled: true
  version: 1
EOF
```

Это приведет к тому, что на всех узлах кластера будут запущены служебные поды компонентов `sds-local-volume`.
Чтобы проверить состояние модуля, выполните следующую команду:

```shell
d8 k get modules sds-local-volume -w
```

В результате будет выведена информация о модуле `sds-local-volume`:

```console
NAME               STAGE   SOURCE      PHASE       ENABLED   READY
sds-local-volume           deckhouse   Available   True      True
```

Чтобы проверить, что в пространстве имен `d8-sds-local-volume` и `d8-sds-node-configurator` все поды в состоянии `Running` или `Completed`, и запущены на всех узлах, где планируется использовать ресурсы LVM, можно использовать команды:

```shell
d8 k -n d8-sds-local-volume get pod -w
d8 k -n d8-sds-node-configurator get pod -w
```

## Преднастройка узлов

### Создание групп томов LVM

Убедитесь, что на всех узлах, где планируется использовать ресурсы LVM, запущены служебные поды `sds-local-volume-csi-node`, которые обеспечивают взаимодействие с узлами, на которых расположены компоненты LVM.

Сделать это можно с помощью команды:

```shell
d8 k -n d8-sds-local-volume get pod -l app=sds-local-volume-csi-node -owide
```

Размещение данных подов по узлам определяется на основе специальных меток (nodeSelector), которые указываются в поле `spec.settings.dataNodes.nodeSelector` в настройках модуля.

Перед тем как приступить к настройке создания объектов StorageClass, необходимо объединить доступные на узлах блочные устройства в группы томов LVM. В дальнейшем группы томов будут использоваться для размещения ресурсов `PersistentVolume`.
Чтобы получить доступные блочные устройства, можно использовать ресурс `BlockDevices`, который отражает их актуальное состояние:

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

В примере выполнения команды выше в наличии имеется шесть блочных устройств, расположенных на трех узлах.

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
  # Раскомментируйте, если важно иметь возможность создавать Thin-хранилища, детали будут раскрыты далее.
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

Если ресурс перешел в состояние `Ready`, то это значит, что на узле worker-0 из блочных устройств `/dev/nvme1n1` и `/dev/nvme0n1p6` была создана группа томов LVM с именем `vg`.

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

### Создание StorageClass

### StorageClass с типом Thick

Создание объектов StorageClass осуществляется через ресурс `LocalStorageClass`, который определяет конфигурацию для желаемого класса хранения. Ручное создание ресурса StorageClass без `LocalStorageClass` может привести к ошибкам.

При создании `LocalStorageClass` важно выбрать тип хранения, который может иметь значение `Thick`, либо `Thin`.

Thick-пул обладает высокой производительностью, сравнимой с производительностью накопителя, но не позволяет использовать снапшоты, в то время как Thin-пул позволит использовать снапшоты и overprovisioning (сверхвыделение ресурсов), но производительность будет ниже.

{% alert level="warning" %}
Overprovisioning нужно использовать с осторожностью, контролируя наличие свободного места в пуле (в системе мониторинга кластера есть отдельные события при достижении 20%, 10%, 5% и 1% свободного места в пуле). При отсутствии свободного места в пуле будет наблюдаться деградация в работе модуля в целом, а также существует реальная вероятность потери данных.
{% endalert %}

Пример создания ресурса `LocalStorageClass` с типом `Thick`:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: LocalStorageClass
metadata:
  name: local-storage-class-thick
spec:
  lvm:
    lvmVolumeGroups:
      - name: vg-on-worker-0
      - name: vg-on-worker-1
      - name: vg-on-worker-2
    type: Thick
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
EOF
```

Проверьте, что созданный `LocalStorageClass` перешел в состояние `Created`, выполнив следующую команду:

```shell
d8 k get lsc local-storage-class -w
```

В результате будет выведена информация о созданном `LocalStorageClass`:

```console
NAME                        PHASE     AGE
local-storage-class-thick   Created   1h
```

Убедитесь, что был создан соответствующий StorageClass, выполнив следующую команду:

```shell
d8 k get sc local-storage-class
```

В результате будет выведена информация о созданном StorageClass:

```console
NAME                        PROVISIONER                      RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
local-storage-class-thick   local.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   1h
```

### StorageClass с типом Thin

Созданные ранее LVMVolumeGroup подходят для создания Thick-хранилищ. Если вам важно иметь возможность создавать хранилища с типом `Thin`, обновите конфигурацию ресурсов `LVMVolumeGroup`, добавив определение для Thin-пула:

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

В обновленной версии `LVMVolumeGroup` 70% доступного пространства будет использовано для создания Thin-хранилищ. Оставшиеся 30% могут быть использованы для Thick-хранилищ.

Повторите добавление Thin-пулов для оставшихся узлов (worker-1 и worker-2).
Пример создания ресурса `LocalStorageClass` с типом Thick:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: LocalStorageClass
metadata:
  name: local-storage-class-thin
spec:
  lvm:
    lvmVolumeGroups:
      - name: vg-on-worker-0
        thin:
          - name: thin-pool-0
      - name: vg-on-worker-1
        thin:
          - name: thin-pool-0
      - name: vg-on-worker-2
        thin:
          - name: thin-pool-0
    type: Thin
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
EOF
```

Проверьте, что созданный `LocalStorageClass` перешел в состояние `Created`, выполнив следующую команду:

```shell
d8 k get lsc local-storage-class -w
```

В результате будет выведена информация о созданном `LocalStorageClass`:

```console
NAME                       PHASE     AGE
local-storage-class-thin   Created   1h
```

Убедитесь, что был создан соответствующий StorageClass, выполнив следующую команду:

```shell
d8 k get sc local-storage-class
```

В результате будет выведена информация о созданном StorageClass:

```console
NAME                       PROVISIONER                      RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
local-storage-class-thin   local.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   1h
```
