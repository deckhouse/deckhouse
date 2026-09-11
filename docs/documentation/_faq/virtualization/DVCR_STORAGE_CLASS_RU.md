---
title: Как сменить StorageClass у DVCR, если PVC уже создан?
subsystems:
- virtualization
lang: ru
---

{% alert level="warning" %}
StorageClass хранилища DVCR можно сменить только пересозданием PVC. При этом теряются все ранее загруженные в DVCR образы, то есть существующие ресурсы [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) и [VirtualImage](/modules/virtualization/cr.html#virtualimage) фактически перестают соответствовать данным в хранилище.
{% endalert %}

Поле [`spec.settings.dvcr.storage.persistentVolumeClaim.storageClassName`](/modules/virtualization/configuration.html#parameters-dvcr-storage-persistentvolumeclaim-storageclassname) в ModuleConfig модуля `virtualization` задаёт класс хранения тома хранилища образов виртуальных машин (DVCR). Пока в пространстве имён `d8-virtualization` существует PVC этого тома, изменить поле через API нельзя.

У уже созданного PVC в Kubernetes нельзя сменить `storageClassName`, а штатного переноса данных DVCR между классами хранения нет.

Чтобы сменить StorageClass у DVCR, выполните следующие шаги:

1. Остановите DVCR:

   ```shell
   d8 k -n d8-virtualization scale deployment dvcr --replicas=0
   ```

1. Выведите список PVC в неймспейсе `d8-virtualization` и найдите PVC тома DVCR:

   ```shell
   d8 k get pvc -n d8-virtualization
   ```

1. Удалите найденный PVC, подставив имя ресурса вместо `<PVC_NAME>`. Если команда завершается ошибкой из-за недостаточных прав, выполните её от имени `system:sudouser`:

   ```shell
   d8 k --as system:sudouser -n d8-virtualization delete pvc/<PVC_NAME>
   ```

1. Задайте новый StorageClass в ModuleConfig, подставив нужный класс вместо `<STORAGE_CLASS_NAME>`:

   ```shell
   d8 k patch mc virtualization --type merge -p '{"spec":{"settings":{"dvcr":{"storage":{"persistentVolumeClaim":{"storageClassName":"<STORAGE_CLASS_NAME>"}}}}}}'
   ```

   Пример вывода:

   ```console
   moduleconfig.deckhouse.io/virtualization patched
   ```

1. Запустите DVCR:

   ```shell
   d8 k -n d8-virtualization scale deployment dvcr --replicas=1
   ```

1. Проверьте PVC:

   ```shell
   d8 k get pvc -n d8-virtualization
   ```

   Пример вывода:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME   STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS          VOLUMEATTRIBUTESCLASS   AGE
   dvcr   Bound    pvc-b43f2e33-32cc-435a-aa1d-b53df35b030a   100Gi      RWO            linstor-thin-r1-hdd   <unset>                 34s
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

{% alert level="warning" %}
Хранилище выбранного StorageClass должно быть доступно на узлах, где запускается DVCR, то есть на system-узлах либо на worker-узлах, если system-узлов в кластере нет.
{% endalert %}
