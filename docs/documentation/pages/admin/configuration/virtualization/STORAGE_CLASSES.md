---
title: "Storage for virtual machine disks and images"
permalink: en/admin/configuration/virtualization/storage-classes.html
description: "Restricting the list of storage classes available for virtual machine disks and images, and choosing the default class."
search: storage classes, StorageClass, allowedStorageClassSelector, VM disks
---

The project owner chooses the storage class for an image or a disk. You can limit that choice and set a default class. The [`.spec.settings.virtualImages`](/modules/virtualization/configuration.html#parameters-virtualimages) block covers images, and the [`.spec.settings.virtualDisks`](/modules/virtualization/configuration.html#parameters-virtualdisks) block covers disks.

Example:

```yaml
spec:
  settings:
    virtualImages:
      allowedStorageClassSelector:
        matchNames:
          - sc-1
          - sc-2
      defaultStorageClassName: sc-1
    virtualDisks:
      allowedStorageClassSelector:
        matchNames:
          - sc-3
      defaultStorageClassName: sc-3
```

Both blocks work the same way and both are optional. The `allowedStorageClassSelector.matchNames` parameter lists the classes allowed in the [VirtualImage](/modules/virtualization/cr.html#virtualimage) and [VirtualDisk](/modules/virtualization/cr.html#virtualdisk) specification, and `defaultStorageClassName` sets the class for resources where the [`.spec.persistentVolumeClaim.storageClassName`](/modules/virtualization/cr.html#virtualdisk-v1alpha2-spec-persistentvolumeclaim-storageclassname) parameter isn't set.
