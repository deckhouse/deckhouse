---
title: "Настройка локального хранилища на основе LVM"
permalink: ru/virtualization-platform/documentation/admin/platform-management/storage/sds/lvm-local.html
lang: ru
---

Локальное хранилище снижает сетевые задержки и обеспечивает более высокую производительность по сравнению с удалёнными хранилищами, доступ к которым осуществляется по сети. Такой подход особенно эффективен в тестовых средах и EDGE-кластерах. Данная функциональность обеспечивается модулем `sds-local-volume`.

## Настройка локального хранилища

Для корректной работы модуля `sds-local-volume` выполните следующие шаги:

1. Настройте LVMVolumeGroup. Перед созданием StorageClass необходимо создать ресурс [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) модуля `sds-node-configurator` на узлах кластера.
1. Включите модуль `sds-node-configurator`. Убедитесь, что модуль включен **до** включения модуля `sds-local-volume`.
1. Создайте соответствующие StorageClass'ы. Создание StorageClass для CSI-драйвера `local.csi.storage.deckhouse.io` пользователем **запрещено**.

Модуль поддерживает два режима работы: LVM и LVMThin.

## Быстрый старт

Все команды выполняются на машине с доступом к API Kubernetes и правами администратора.

### Включение модулей

Включение модуля `sds-node-configurator`:

1. Создайте ресурс ModuleConfig для включения модуля:

   ```shell
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

1. Дождитесь состояния модуля `Ready`. На этом этапе не требуется проверять поды в пространстве имен `d8-sds-node-configurator`.

   ```shell
   d8 k get modules sds-node-configurator -w
   ```

Включение модуля `sds-local-volume`:

1. Активируйте модуль `sds-local-volume`. Пример ниже запускает модуль с настройками по умолчанию, что приведет к созданию служебных подов компонента `sds-local-volume` на всех узлах кластера:

   ```shell
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

1. Дождитесь состояния модуля `Ready`.

   ```shell
   d8 k get modules sds-local-volume -w
   ```

1. Убедитесь, что в пространствах имен `d8-sds-local-volume` и `d8-sds-node-configurator` все поды находятся в статусе `Running` или `Completed` и запущены на всех узлах, где планируется использовать ресурсы LVM.

   ```shell
   d8 k -n d8-sds-local-volume get pod -owide -w
   d8 k -n d8-sds-node-configurator get pod -o wide -w
   ```

### Подготовка узлов к созданию хранилищ

Для корректной работы хранилищ на узлах необходимо, чтобы поды `sds-local-volume-csi-node` были запущены на выбранных узлах.

По умолчанию эти поды запускаются на всех узлах кластера. Проверить их наличие можно с помощью команды:

```shell
d8 k -n d8-sds-local-volume get pod -owide
```

Размещение подов `sds-local-volume-csi-node` управляется специальными метками (`nodeSelector`). Эти метки задаются в параметре [`spec.settings.dataNodes.nodeSelector`](/modules/sds-local-volume/stable/configuration.html#parameters-datanodes-nodeselector) модуля.

### Настройка хранилища на узлах

Для настройки хранилища на узлах необходимо создать группы томов LVM с использованием ресурсов [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup). В данном примере создается хранилище Thick.

{% alert level="warning" %}
Перед созданием ресурса [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) убедитесь, что на данном узле запущен под `sds-local-volume-csi-node`. Это можно сделать командой:

```shell
d8 k -n d8-sds-local-volume get pod -owide
```

{% endalert %}

1. Получите все ресурсы [BlockDevice](/modules/sds-node-configurator/stable/cr.html#blockdevice), которые доступны в вашем кластере:

   ```shell
   d8 k get bd
   ```

   Пример вывода:

   ```console
   NAME                                           NODE       CONSUMABLE   SIZE           PATH
   dev-ef4fb06b63d2c05fb6ee83008b55e486aa1161aa   worker-0   false        976762584Ki    /dev/nvme1n1
   dev-0cfc0d07f353598e329d34f3821bed992c1ffbcd   worker-0   false        894006140416   /dev/nvme0n1p6
   dev-7e4df1ddf2a1b05a79f9481cdf56d29891a9f9d0   worker-1   false        976762584Ki    /dev/nvme1n1
   dev-b103062f879a2349a9c5f054e0366594568de68d   worker-1   false        894006140416   /dev/nvme0n1p6
   dev-53d904f18b912187ac82de29af06a34d9ae23199   worker-2   false        976762584Ki    /dev/nvme1n1
   dev-6c5abbd549100834c6b1668c8f89fb97872ee2b1   worker-2   false        894006140416   /dev/nvme0n1p6
   ```

1. Создайте ресурс [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) для узла `worker-0`:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: LVMVolumeGroup
   metadata:
     name: "vg-1-on-worker-0" # Подходит любое допустимое имя ресурса в Kubernetes. Это имя ресурса LVMVolumeGroup будет использоваться для создания LocalStorageClass в будущем.
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
     actualVGNameOnTheNode: "vg-1" # Имя LVM VG, который будет создан из указанных блочных устройств на узле.
   EOF
   ```

1. Дождитесь, когда созданный ресурс [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) перейдет в состояние `Ready`:

   ```shell
   d8 k get lvg vg-1-on-worker-0 -w
   ```

   Если ресурс перешел в состояние `Ready`, это значит, что на узле `worker-0` из блочных устройств `/dev/nvme1n1` и `/dev/nvme0n1p6` была создана LVM VG с именем `vg-1`.

1. Создайте ресурс [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) для узла `worker-1`:

   ```shell
   d8 k apply -f - <<EOF
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

1. Дождитесь, когда созданный ресурс [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) перейдет в состояние `Ready`:

   ```shell
   d8 k get lvg vg-1-on-worker-1 -w
   ```

   Если ресурс перешел в состояние `Ready`, это значит, что на узле `worker-1` из блочного устройства `/dev/nvme1n1` и `/dev/nvme0n1p6` была создана LVM VG с именем `vg-1`.

1. Создайте ресурс [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) для узла `worker-2`:

   ```shell
   d8 k apply -f - <<EOF
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

1. Дождитесь, когда созданный ресурс [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) перейдет в состояние `Ready`:

   ```shell
   d8 k get lvg vg-1-on-worker-2 -w
   ```

   Если ресурс перешел в состояние `Ready`, то это значит, что на узле `worker-2` из блочного устройства `/dev/nvme1n1` и `/dev/nvme0n1p6` была создана LVM VG с именем `vg-1`.

1. Создайте ресурс [LocalStorageClass](/modules/sds-local-volume/stable/cr.html#localstorageclass):

   ```shell
   d8 k apply -f -<<EOF
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

   Для thin volume:

   ```shell
   d8 k apply -f -<<EOF
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
           thin:
             poolName: thin-1
       type: Thin
     reclaimPolicy: Delete
     volumeBindingMode: WaitForFirstConsumer
   EOF
   ```

   > **Важно.** В [LocalStorageClass](/modules/sds-local-volume/stable/cr.html#localstorageclass) с `type: Thick` нельзя использовать LocalVolumeGroup, содержащие хотя бы один thin pool.

1. Дождитесь, когда созданный ресурс LocalStorageClass перейдет в состояние `Created`:

   ```shell
   d8 k get lsc local-storage-class -w
   ```

1. Проверьте, что был создан соответствующий StorageClass:

   ```shell
   d8 k get sc local-storage-class
   ```

Если StorageClass с именем `local-storage-class` появился, значит настройка модуля `sds-local-volume` завершена. Теперь пользователи могут создавать PVC, указывая StorageClass с именем `local-storage-class`.

### Выбор метода очистки тома после удаления PV

При удалении файлов операционная система не удаляет содержимое физически, а лишь помечает соответствующие блоки как «свободные». Если новый том получает физические блоки, ранее использовавшиеся другим томом, в них могут остаться данные предыдущего пользователя.

Это возможно, например, в таком случае:

- пользователь №1 разместил файлы в томе, запрошенном из StorageClass 1 и на узле 1 (неважно, в режиме «Block» или «Filesystem»);
- пользователь №1 удалил файлы и том;
- физические блоки, которые он занимал, становятся «свободными», но не затертыми;
- пользователь №2 запросил новый том из StorageClass 1 и на узле 1 в режиме «Block»;
- есть риск, что часть или все блоки, ранее занимаемые пользователем №1, будут снова выделены пользователю №2;
- в этом случае пользователь №2 имеет возможность восстановить данные пользователя №1.

### Thick-тома

Для предотвращения утечек через thick-тома предусмотрен параметр `volumeCleanup`. Он позволяет выбрать метод очистки тома перед удалением PV.

Возможные значения:

- Параметр не задан — не выполнять никаких дополнительных действий при удалении тома. Данные могут оказаться доступными следующему пользователю;
- `RandomFillSinglePass` — том будет перезаписан случайными данными один раз перед удалением. Использовать эту опцию не рекомендуется для твердотельных накопителей, так как она уменьшает ресурс накопителя;
- `RandomFillThreePass` — том будет перезаписан случайными данными три раза перед удалением. Использовать эту опцию не рекомендуется для твердотельных накопителей, так как она уменьшает ресурс накопителя;
- `Discard` — все блоки тома будут отмечены как свободные с использованием системного вызова `discard` перед удалением. Эта опция имеет смысл только для твердотельных накопителей.

  Большинство современных твердотельных накопителей гарантирует, что помеченный `discard` блок при чтении не вернет предыдущие данные. Это делает опцию `Discard` самым эффективным способом предотвращения утечек при использовании твердотельных накопителей.

  Однако очистка ячейки — относительно долгая операция, поэтому выполняется устройством в фоне. К тому же, многие диски не могут очищать индивидуальные ячейки, а только группы (страницы). Из-за этого не все накопители гарантируют немедленную недоступность освобожденных данных. Кроме того, не все накопители, гарантирующие это, держат обещание. Если устройство не гарантирует Deterministic TRIM (DRAT), Deterministic Read Zero after TRIM (RZAT) и не является проверенным, то использовать его не рекомендуется.

### Thin-тома

В момент освобождения блока thin-тома через `discard` гостевой операционной системы эта команда пересылается на устройство. В случае использования жесткого диска или при отсутствии поддержки `discard` со стороны твердотельного накопителя данные могут остаться на thin pool до нового использования такого блока. Однако, пользователям предоставляется доступ только к thin-томам, а не к самому thin pool. Они могут получить только том из пула, а для thin-томов производится обнуление блока thin pool при новом использовании, что предотвращает утечки между клиентами. Это гарантируется настройкой `thin_pool_zero=1` в LVM.

## Системные требования и рекомендации

- Используйте стоковые ядра, поставляемые вместе с [поддерживаемыми дистрибутивами](/products/virtualization-platform/documentation/about/requirements.html).
- Не используйте другой SDS (Software defined storage) для предоставления дисков SDS DVP.
