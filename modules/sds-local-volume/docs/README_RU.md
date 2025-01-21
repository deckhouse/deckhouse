---
title: "Модуль sds-local-volume"
description: "Модуль sds-local-volume: общие концепции и положения."
moduleStatus: preview
---

Модуль предназначен для управления локальным блочным хранилищем на базе LVM. С его помощью можно создавать StorageClass в Kubernetes, используя ресурс [LocalStorageClass](cr.html#localstorageclass).

## Шаги настройки модуля

Для корректной работы модуля `sds-local-volume` выполните следующие шаги:

- Настройте LVMVolumeGroup.

  Перед созданием StorageClass необходимо создать ресурс [LVMVolumeGroup](../../sds-node-configurator/stable/cr.html#lvmvolumegroup) модуля `sds-node-configurator` на узлах кластера.

- Включите модуль [sds-node-configurator](../../sds-node-configurator/stable/).

  Убедитесь, что модуль `sds-node-configurator` включен **до** включения модуля `sds-local-volume`.

- Создайте соответствующие StorageClass'ы.

  Создание StorageClass для CSI-драйвера `local.csi.storage.deckhouse.io` пользователем **запрещено**.

Модуль поддерживает два режима работы: LVM и LVMThin.
У каждого из них есть свои особенности, преимущества и ограничения. Подробнее о различиях можно узнать в [FAQ](./faq.html#когда-следует-использовать-lvm-а-когда-lvmthin).

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

{{< alert level="warning" >}}
Перед созданием ресурса LVMVolumeGroup убедитесь, что на данном узле запущен под `sds-local-volume-csi-node`. Это можно сделать командой:

```shell
kubectl -n d8-sds-local-volume get pod -owide
```

{{< /alert >}}

#### Шаги настройки

1. Получите все ресурсы [BlockDevice](../../sds-node-configurator/stable/cr.html#blockdevice), которые доступны в вашем кластере:

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

1. Создайте ресурс [LVMVolumeGroup](../../sds-node-configurator/stable/cr.html#lvmvolumegroup) для узла `worker-0`:

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
     actualVGNameOnTheNode: "vg-1" # the name of the LVM VG to be created from the above block devices on the node 
   EOF
   ```

1. Дождитесь, когда созданный ресурс LVMVolumeGroup перейдет в состояние `Ready`:

   ```shell
   kubectl get lvg vg-1-on-worker-0 -w
   ```

   Если ресурс перешел в состояние `Ready`, это значит, что на узле `worker-0` из блочных устройств `/dev/nvme1n1` и `/dev/nvme0n1p6` была создана LVM VG с именем `vg-1`.

1. Создайте ресурс [LVMVolumeGroup](../../sds-node-configurator/stable/cr.html#lvmvolumegroup) для узла `worker-1`:

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

1. Создайте ресурс [LVMVolumeGroup](../../sds-node-configurator/stable/cr.html#lvmvolumegroup) для узла `worker-2`:

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

1. Дождитесь, когда созданный ресурс LocalStorageClass перейдет в состояние `Created`:

   ```shell
   kubectl get lsc local-storage-class -w
   ```

1. Проверьте, что соответствующий StorageClass создался:

   ```shell
   kubectl get sc local-storage-class
   ```

Если StorageClass с именем `local-storage-class` появился, значит настройка модуля `sds-local-volume` завершена. Теперь пользователи могут создавать PVC, указывая StorageClass с именем `local-storage-class`.

## Системные требования и рекомендации

- Используйте стоковые ядра, поставляемые вместе с [поддерживаемыми дистрибутивами](https://deckhouse.ru/documentation/v1/supported_versions.html#linux).
- Не используйте другой SDS (Software defined storage) для предоставления дисков SDS Deckhouse.
