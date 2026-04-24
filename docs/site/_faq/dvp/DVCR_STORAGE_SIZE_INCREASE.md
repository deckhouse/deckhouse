---
title: How to increase the DVCR size?
sections:
- platform_management
lang: en
---

The DVCR volume size is set in the `virtualization` module ModuleConfig (`spec.settings.dvcr.storage.persistentVolumeClaim.size`). The new value must be greater than the current one.

1. Check the current DVCR size:

   ```shell
   d8 k get mc virtualization -o jsonpath='{.spec.settings.dvcr.storage.persistentVolumeClaim}'
   ```

   Example output:

   ```console
    {"size":"58G","storageClass":"linstor-thick-data-r1"}
   ```

1. Increase `size` using `patch` (set the value you need):

   ```shell
   d8 k patch mc virtualization \
     --type merge -p '{"spec": {"settings": {"dvcr": {"storage": {"persistentVolumeClaim": {"size":"59G"}}}}}}'
   ```

   Example output:

   ```console
   moduleconfig.deckhouse.io/virtualization patched
   ```

1. Verify that ModuleConfig shows the new size:

   ```shell
   d8 k get mc virtualization -o jsonpath='{.spec.settings.dvcr.storage.persistentVolumeClaim}'
   ```

   Example output:

   ```console
   {"size":"59G","storageClass":"linstor-thick-data-r1"}
   ```

1. Check the current DVCR status:

   ```shell
   d8 k get pvc dvcr -n d8-virtualization
   ```

   Example output:

   ```console
   NAME STATUS VOLUME                                    CAPACITY    ACCESS MODES   STORAGECLASS           AGE
   dvcr Bound  pvc-6a6cedb8-1292-4440-b789-5cc9d15bbc6b  57617188Ki  RWO            linstor-thick-data-r1  7d
   ```
