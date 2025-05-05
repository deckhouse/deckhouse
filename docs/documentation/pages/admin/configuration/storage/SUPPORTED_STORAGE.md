---
title: "Storage configuration"
permalink: en/admin/storage/supported-storage.html
---

Storage setup involves several steps depending on the selected [storage type](../storage/overview.html#supported-storage-types). The main configuration steps include:

- Enabling and configuring the corresponding modules.
- Creating Volume Groups.
- Preparing and creating StorageClass objects, followed by assigning and using them.

Each storage type may have its own specific requirements and configuration details, which are described in the corresponding guides.

## Creating a StorageClass

To create StorageClass objects, you need to connect one or more storage backends that manage PersistentVolume resources. The created StorageClass objects can then be used to provision virtual disks and images. For more information on creating and using StorageClasses, refer to the appropriate documentation sections for each [storage type](../storage/overview.html#supported-storage-types).

## Setting a Default StorageClass

The default StorageClass is used when a PersistentVolumeClaim is created without explicitly specifying a storage class. This simplifies the process of creating and using storage by eliminating the need to define the class manually each time.

To set the default StorageClass, specify the desired class in the global configuration. Example command:

```shell
# Replace with the name of your StorageClass object.
DEFAULT_STORAGE_CLASS=replicated-storage-class
d8 k patch mc global --type='json' -p='[{"op": "replace", "path": "/spec/settings/defaultClusterStorageClass", "value": "'"$DEFAULT_STORAGE_CLASS"'"}]'
```
