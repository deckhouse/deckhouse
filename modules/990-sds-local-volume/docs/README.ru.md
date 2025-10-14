---
title: "Модуль sds-local-volume"
description: "Модуль sds-local-volume: общие концепции и положения."
---

Модуль предназначен для управления локальным блочным хранилищем на базе LVM. С его помощью можно создавать StorageClass в Kubernetes, используя ресурс [LocalStorageClass](cr.html#localstorageclass).

## Шаги настройки модуля

Для корректной работы модуля `sds-local-volume` выполните следующие шаги:

- Настройте LVMVolumeGroup.

  Перед созданием StorageClass необходимо создать ресурс [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) модуля `sds-node-configurator` на узлах кластера.

- Включите модуль [sds-node-configurator](/sds-node-configurator/).

  Убедитесь, что модуль `sds-node-configurator` включен **до** включения модуля `sds-local-volume`.

- Создайте соответствующие StorageClass'ы.

  Создание StorageClass для CSI-драйвера `local.csi.storage.deckhouse.io` пользователем **запрещено**.

{% alert level="info" %}
Для работы с снапшотами требуется подключенный модуль [snapshot-controller](/modules/snapshot-controller/).
{% endalert %}

Модуль поддерживает два режима работы: LVM (Thick) и LVM Thin.
У каждого из них есть свои особенности, преимущества и ограничения. Подробнее о различиях можно узнать в [FAQ](./faq.html#когда-следует-использовать-lvm-а-когда-lvm-thin).

## Быстрый старт

Все команды выполняются на машине с доступом к API Kubernetes и правами администратора.

### Включение модулей

Включение модуля `sds-node-configurator`:

1. Создайте ресурс ModuleConfig для включения модуля:

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: sds-node-configurator
   spec:
     enabled: true
     version: 1
   EOF
   ```

1. Дождитесь состояния модуля `Ready`. На этом этапе не требуется проверять поды в пространстве имен `d8-sds-node-configurator`.

   ```shell
   kubectl get modules sds-node-configurator -w
   ```

Включение модуля `sds-local-volume`:

1. Активируйте модуль `sds-local-volume`. Перед включением рекомендуется ознакомиться с [доступными настройками](./configuration.html). Пример ниже запускает модуль с настройками по умолчанию, что приведет к созданию служебных подов компонента `sds-local-volume` на всех узлах кластера:

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: sds-local-volume
   spec:
     enabled: true
     version: 1
   EOF
   ```

1. Дождитесь состояния модуля `Ready`.

   ```shell
   kubectl get modules sds-local-volume -w
   ```

1. Убедитесь, что в пространствах имен `d8-sds-local-volume` и `d8-sds-node-configurator` все поды находятся в статусе `Running` или `Completed` и запущены на всех узлах, где планируется использовать ресурсы LVM.

   ```shell
   kubectl -n d8-sds-local-volume get pod -owide -w
   kubectl -n d8-sds-node-configurator get pod -o wide -w
   ```

### Подготовка узлов к созданию хранилищ на них

Для корректной работы хранилищ на узлах необходимо, чтобы поды `sds-local-volume-csi-node` были запущены на выбранных узлах.

По умолчанию эти поды запускаются на всех узлах кластера. Проверить их наличие можно с помощью команды:

```shell
kubectl -n d8-sds-local-volume get pod -owide
```

Размещение подов `sds-local-volume-csi-node` управляется специальными метками (nodeSelector). Эти метки задаются в параметре [spec.settings.dataNodes.nodeSelector](configuration.html#parameters-datanodes-nodeselector) модуля. Подробнее о настройке и выборе узлов для работы модуля можно узнать [в FAQ](./faq.html#я-не-хочу-чтобы-модуль-использовался-на-всех-узлах-кластера-как-мне-выбрать-желаемые-узлы).

### Настройка хранилища на узлах

Для настройки хранилища на узлах необходимо создать группы томов LVM с использованием ресурсов LVMVolumeGroup. В данном примере создается хранилище Thick.

{% alert level="warning" %}
Перед созданием ресурса LVMVolumeGroup убедитесь, что на данном узле запущен под `sds-local-volume-csi-node`. Это можно сделать командой:

```shell
kubectl -n d8-sds-local-volume get pod -owide
```

{% endalert %}

#### Шаги настройки

1. Получите все ресурсы [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice), которые доступны в вашем кластере:

   ```shell
   kubectl get bd

   NAME                                           NODE       CONSUMABLE   SIZE           PATH
   dev-ef4fb06b63d2c05fb6ee83008b55e486aa1161aa   worker-0   false        976762584Ki    /dev/nvme1n1
   dev-0cfc0d07f353598e329d34f3821bed992c1ffbcd   worker-0   false        894006140416   /dev/nvme0n1p6
   dev-7e4df1ddf2a1b05a79f9481cdf56d29891a9f9d0   worker-1   false        976762584Ki    /dev/nvme1n1
   dev-b103062f879a2349a9c5f054e0366594568de68d   worker-1   false        894006140416   /dev/nvme0n1p6
   dev-53d904f18b912187ac82de29af06a34d9ae23199   worker-2   false        976762584Ki    /dev/nvme1n1
   dev-6c5abbd549100834c6b1668c8f89fb97872ee2b1   worker-2   false        894006140416   /dev/nvme0n1p6
   ```

1. Создайте ресурс [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) для узла `worker-0`:

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: LVMVolumeGroup
   metadata:
     name: "vg-1-on-worker-0" # The name can be any fully qualified resource name in Kubernetes. This LVMVolumeGroup resource name will be used to create LocalStorageClass in the future
   spec:
     type: Local
     local:
       nodeName: "worker-0"
     blockDeviceSelector:
       matchExpressions:
         - key: kubernetes.io/metadata.name
           operator: In
           values:
             - dev-ef4fb06b63d2c05fb6ee83008b55e486aa1161aa
             - dev-0cfc0d07f353598e329d34f3821bed992c1ffbcd
     actualVGNameOnTheNode: "vg-1" # Имя LVM VG, который будет создан из указанных блочных устройств на узле
   EOF
   ```

1. Дождитесь, когда созданный ресурс LVMVolumeGroup перейдет в состояние `Ready`:

   ```shell
   kubectl get lvg vg-1-on-worker-0 -w
   ```

   Если ресурс перешел в состояние `Ready`, это значит, что на узле `worker-0` из блочных устройств `/dev/nvme1n1` и `/dev/nvme0n1p6` была создана LVM VG с именем `vg-1`.

1. Создайте ресурс [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) для узла `worker-1`:

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: LVMVolumeGroup
   metadata:
     name: "vg-1-on-worker-1"
   spec:
     type: Local
     local:
       nodeName: "worker-1"
     blockDeviceSelector:
       matchExpressions:
         - key: kubernetes.io/metadata.name
           operator: In
           values:
             - dev-7e4df1ddf2a1b05a79f9481cdf56d29891a9f9d0
             - dev-b103062f879a2349a9c5f054e0366594568de68d
     actualVGNameOnTheNode: "vg-1"
   EOF
   ```

1. Дождитесь, когда созданный ресурс LVMVolumeGroup перейдет в состояние `Ready`:

   ```shell
   kubectl get lvg vg-1-on-worker-1 -w
   ```

   Если ресурс перешел в состояние `Ready`, это значит, что на узле `worker-1` из блочного устройства `/dev/nvme1n1` и `/dev/nvme0n1p6` была создана LVM VG с именем `vg-1`.

1. Создайте ресурс [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) для узла `worker-2`:

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: LVMVolumeGroup
   metadata:
     name: "vg-1-on-worker-2"
   spec:
     type: Local
     local:
       nodeName: "worker-2"
     blockDeviceSelector:
       matchExpressions:
         - key: kubernetes.io/metadata.name
           operator: In
           values:
             - dev-53d904f18b912187ac82de29af06a34d9ae23199
             - dev-6c5abbd549100834c6b1668c8f89fb97872ee2b1
     actualVGNameOnTheNode: "vg-1"
   EOF
   ```

1. Дождитесь, когда созданный ресурс LVMVolumeGroup перейдет в состояние `Ready`:

   ```shell
   kubectl get lvg vg-1-on-worker-2 -w
   ```

   Если ресурс перешел в состояние `Ready`, то это значит, что на узле `worker-2` из блочного устройства `/dev/nvme1n1` и `/dev/nvme0n1p6` была создана LVM VG с именем `vg-1`.

1. Создайте ресурс [LocalStorageClass](./cr.html#localstorageclass):

   ```yaml
   kubectl apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: LocalStorageClass
   metadata:
     name: local-storage-class
   spec:
     lvm:
       lvmVolumeGroups:
         - name: vg-1-on-worker-0
         - name: vg-1-on-worker-1
         - name: vg-1-on-worker-2
       type: Thick
     reclaimPolicy: Delete
     volumeBindingMode: WaitForFirstConsumer
   EOF
   ```

   Для thin volume

   ```yaml
   kubectl apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: LocalStorageClass
   metadata:
     name: local-storage-class
   spec:
     lvm:
       lvmVolumeGroups:
        - name: vg-1-on-worker-0
          thin:
            poolName: thin-1
        - name: vg-1-on-worker-1
          thin:
            poolName: thin-1
        - name: vg-1-on-worker-2
          thin:
            poolName: thin-1
       type: Thin
     reclaimPolicy: Delete
     volumeBindingMode: WaitForFirstConsumer
   EOF
   ```

> **Внимание.** В `LocalStorageClass` с `type: Thick` нельзя использовать LocalVolumeGroup, содержащие хотя бы один thin pool.

1. Дождитесь, когда созданный ресурс LocalStorageClass перейдет в состояние `Created`:

   ```shell
   kubectl get lsc local-storage-class -w
   ```

1. Проверьте, что соответствующий StorageClass создался:

   ```shell
   kubectl get sc local-storage-class
   ```

Если StorageClass с именем `local-storage-class` появился, значит настройка модуля `sds-local-volume` завершена. Теперь пользователи могут создавать PVC, указывая StorageClass с именем `local-storage-class`.

### Выбор метода очистки тома после удаления PV

При удалении файлов операционная система не удаляет содержимое физически, а лишь помечает соответствующие блоки как «свободные». Если новый том получает физические блоки, ранее использовавшиеся другим томом, в них могут остаться данные предыдущего пользователя.

Это возможно, например, в таком случае:

- пользователь №1 разместил файлы в томе, запрошенном из StorageClass 1 и на узле 1 (не важно, в режиме «Block» или «Filesystem»);
- пользователь №1 удалил файлы и том;
- физические блоки, которые он занимал, становятся «свободными», но не затертыми;
- пользователь №2 запросил новый том из StorageClass 1 и на узле 1 в режиме «Block»;
- есть риск, что часть или все блоки, ранее занимаемые пользователем №1, будут снова выделены пользователю №2;
- в этом случае пользователь №2 имеет возможность восстановить данные пользователя №1.

### Thick-тома

Для предотвращения утечек через thick-тома предусмотрен параметр `volumeCleanup`.
Он позволяет выбрать метод очистки тома перед удалением PV.
Возможные значения:

- параметр не задан — не выполнять никаких дополнительных действий при удалении тома. Данные могут оказаться доступными следующему пользователю;

- `RandomFillSinglePass` — том будет перезаписан случайными данными один раз перед удалением. Использовать эту опцию не рекомендуется для твердотельных накопителей, так как она уменьшает ресурс накопителя.

- `RandomFillThreePass` — том будет перезаписан случайными данными три раза перед удалением. Использовать эту опцию не рекомендуется для твердотельных накопителей, так как она уменьшает ресурс накопителя.

- `Discard` — все блоки тома будут отмечены как свободные с использованием системного вызова `discard` перед удалением. Эта опция имеет смысл только для твердотельных накопителей.

Большинство современных твердотельных накопителей гарантирует, что помеченный `discard` блок при чтении не вернет предыдущие данные. Это делает опцию `Discard` самым эффективным способом предотвращения утечек при использовании твердотельных накопителей.
Однако очистка ячейки относительно долгая операция, поэтому выполняется устройством в фоне. К тому-же многие диски не могут очищать индивидуальные ячейки, а только группы - страницы. Из-за этого не все накопители гарантируют немедленную недоступность освобожденных данных. К тому-же не все накопители, гарантирующие это, держат обещание.
Если устройство не гарантирует Deterministic TRIM (DRAT), Deterministic Read Zero after TRIM (RZAT) и не является проверенным, то использовать его не рекомендуется.

### Thin-тома

В момент освобождения блока thin-тома через `discard` гостевой операционной системы эта команда пересылается на устройство. В случае использования жесткого диска или отсутствии поддержки `discard` со стороны твердотельного накопителя данные могут остаться на thin-pool до нового использования такого блока. Однако, пользователям предоставляется доступ только к thin-томам, а не к самому thin-пулу. Они могут получить только том из пула, а для thin-томов производится зануление блока thin-pool при новом использовании, что предотвращает утечки между клиентами. Это гарантируется настройкой `thin_pool_zero=1` в LVM.

## Системные требования и рекомендации

- Используйте стоковые ядра, поставляемые вместе с поддерживаемыми дистрибутивами.
- Не используйте другой SDS (Software defined storage) для предоставления дисков SDS Deckhouse.
