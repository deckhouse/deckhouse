---
title: Как увеличить размер DVCR?
section: platform_management
lang: ru
---

Размер тома DVCR задаётся в ModuleConfig модуля `virtualization` (`spec.settings.dvcr.storage.persistentVolumeClaim.size`). Новое значение должно быть больше текущего.

1. Проверьте текущий размер DVCR:

   ```shell
   d8 k get mc virtualization -o jsonpath='{.spec.settings.dvcr.storage.persistentVolumeClaim}'
   ```

   Пример вывода:

   ```console
    {"size":"58G","storageClass":"linstor-thick-data-r1"}
   ```

1. Увеличьте `size` через `patch` (подставьте нужное значение):

   ```shell
   d8 k patch mc virtualization \
     --type merge -p '{"spec": {"settings": {"dvcr": {"storage": {"persistentVolumeClaim": {"size":"59G"}}}}}}'
   ```

   Пример вывода:

   ```console
   moduleconfig.deckhouse.io/virtualization patched
   ```

1. Убедитесь, что в ModuleConfig отображается новый размер:

   ```shell
   d8 k get mc virtualization -o jsonpath='{.spec.settings.dvcr.storage.persistentVolumeClaim}'
   ```

   Пример вывода:

   ```console
   {"size":"59G","storageClass":"linstor-thick-data-r1"}
   ```

1. Проверьте текущее состояние DVCR:

   ```shell
   d8 k get pvc dvcr -n d8-virtualization
   ```

   Пример вывода:

   ```console
   NAME STATUS VOLUME                                    CAPACITY    ACCESS MODES   STORAGECLASS           AGE
   dvcr Bound  pvc-6a6cedb8-1292-4440-b789-5cc9d15bbc6b  57617188Ki  RWO            linstor-thick-data-r1  7d
   ```
