---
title: "Локальное хранилище на основе LVM"
permalink: ru/storage/admin/sds/lvm-local.html
lang: ru
---

{% alert level="info" %}
<span style="border-bottom: 1px dotted #000;" data-tippy-content="Ограничение на возможность создания снимков">
Доступно с ограничениями в редакциях:</span>  **CE**

Доступно без ограничений в некоторых коммерческих редакциях:  **SE, SE+, EE**

Подробнее см. в разделе [Условия и цены](../../../../../pricing/).
{% endalert %}

Использование локального хранилища помогает избежать сетевых задержек и повышает производительность по сравнению с удалёнными хранилищами, доступ к которым осуществляется по сети. Этот подход идеально подходит для тестовых сред и EDGE-кластеров.

## Включение модуля

Настройка локального блочного хранилища происходит на основе логического менеджера томов LVM (Logical Volume Manager). Управление LVM осуществляется модулем `sds-node-configurator`, который необходимо включить перед активацией модуля `sds-local-volume`.

Чтобы включить модуль, примените ресурс ModuleConfig:

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

Дождитесь, когда модуль `sds-node-configurator` перейдет в состояние `Ready`. Проверить состояние можно, выполнив следующую команду:

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

Это приведет к тому, что на всех узлах кластера будут запущены служебные поды компонентов `sds-local-volume`. Чтобы проверить состояние модуля, выполните следующую команду:

```shell
d8 k get modules sds-local-volume -w
```

В результате будет выведена информация о модуле `sds-local-volume`:

```console
NAME                    STAGE   SOURCE    PHASE       ENABLED   READY
sds-local-volume                          Available   True      True
```

Чтобы проверить, что в пространстве имен `d8-sds-local-volume` и `d8-sds-node-configurator` все поды в состоянии `Running` или `Completed` и запущены на всех узлах, где планируется использовать ресурсы LVM, можно использовать команды:

```shell
d8 k -n d8-sds-local-volume get pod -w
d8 k -n d8-sds-node-configurator get pod -w
```

## Преднастройка узлов

### Создание групп томов LVM

Убедитесь, что на всех узлах, предназначенных для использования ресурсов LVM, запущены сервисные поды `sds-local-volume-csi-node`, обеспечивающие взаимодействие с узлами, содержащими компоненты LVM. Сделать это можно с помощью команды:

```shell
d8 k -n d8-sds-local-volume get pod -l app=sds-local-volume-csi-node -owide
```

Размещение данных подов по узлам определяется на основе специальных меток (`nodeSelector`), которые указываются в поле `spec.settings.dataNodes.nodeSelector` в настройках модуля.

Перед тем как приступить к настройке создания объектов StorageClass, необходимо объединить доступные на узлах блочные устройства в группы томов LVM. В дальнейшем группы томов будут использоваться для размещения ресурсов PersistentVolume.

Чтобы получить доступные блочные устройства, можно использовать ресурс BlockDevices который отражает их актуальное состояние:

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

В примере выполнения команды выше в наличии имеется шесть блочных устройств, расположенных на трех узлах. Чтобы объединить блочные устройства на одном узле, необходимо создать группу томов LVM с помощью ресурса [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/).

Для создания ресурса [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/) на узле worker-0 примените следующий ресурс, предварительно заменив имена узла и блочных устройств на свои:

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
  # Раскомментируйте, если важно иметь возможность создавать thin-хранилища, детали будут раскрыты далее.
  # thinPools:
  #   - name: thin-pool-0
  #     size: 70% 
EOF
```

Дождитесь, когда созданный ресурс [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/) перейдет в состояние `Ready`. Чтобы проверить состояние ресурса, выполните следующую команду:

```shell
d8 k get lvg vg-on-worker-0 -w
```

В результате будет выведена информация о состоянии ресурса:

```console
NAME             THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE       ALLOCATED SIZE   VG   AGE
vg-on-worker-0   1/1         True                    Ready   worker-0   360484Mi   30064Mi          vg   1h
```

Если ресурс перешел в состояние `Ready`, то это значит, что на узле worker-0 из блочных устройств `/dev/nvme1n1` и `/dev/nvme0n1p6` была создана группа томов LVM с именем `vg`.

Далее необходимо повторить создание ресурсов [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/) для оставшихся узлов (worker-1 и worker-2), изменив в примере выше имя ресурса [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/), имя узла и имена блочных устройств, соответствующих узлу. Убедитесь, что группы томов LVM созданы на всех узлах, где планируется их использовать, выполнив следующую команду:

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

### Создание StorageClass с типом thick

Создание объектов StorageClass осуществляется через ресурс [LocalStorageClass](../../../reference/cr/localstorageclass/), который определяет конфигурацию для желаемого класса хранения. Ручное создание ресурса StorageClass без [LocalStorageClass](../../../reference/cr/localstorageclass/) может привести к ошибкам.

При создании [LocalStorageClass](../../../reference/cr/localstorageclass/) важно выбрать тип хранения, который может иметь значение thick, либо thin.

Thick-пул обеспечивает высокую производительность, сопоставимую с производительностью накопителя, но не поддерживает создание снапшотов.

Пример создания ресурса [LocalStorageClass](../../../reference/cr/localstorageclass/) с типом thick:

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

Проверьте, что созданный [LocalStorageClass](../../../reference/cr/localstorageclass/) перешёл в состояние `Created`, выполнив следующую команду:

```shell
d8 k get lsc local-storage-class -w
```

В результате будет выведена информация о созданном [LocalStorageClass](../../../reference/cr/localstorageclass/):

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

### Создание StorageClass с типом thin

В отличие от thick-пула, thin-пул позволяет использовать снапшоты и overprovisioning (сверхвыделение ресурсов), но имеет меньшую производительность.

{% alert level="warning" %}
Overprovisioning следует использовать с осторожностью, контролируя доступное пространство в пуле. В системе мониторинга кластера предусмотрены события при снижении свободного места до 20%, 10%, 5% и 1%. Полное заполнение пула может привести к деградации работы модуля и риску потери данных.
{% endalert %}

Созданные ранее [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/) подходят для создания thick-хранилищ. Если вам важно иметь возможность создавать хранилища с типом thin, обновите конфигурацию ресурсов [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/), добавив определение для thin-пула:

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

В обновленной версии [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/) 70% доступного пространства будет использовано для создания thin-хранилищ. Оставшиеся 30% могут быть использованы для thick-хранилищ.

Повторите добавление thin-пулов для оставшихся узлов (worker-1 и worker-2). Пример создания ресурса [LocalStorageClass](../../../reference/cr/localstorageclass/) с типом thin:

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

Проверьте, что созданный [LocalStorageClass](../../../reference/cr/localstorageclass/) перешёл в состояние `Created`, выполнив следующую команду:

```shell
d8 k get lsc local-storage-class -w
```

В результате будет выведена информация о созданном [LocalStorageClass](../../../reference/cr/localstorageclass/):

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
