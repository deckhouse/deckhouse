---
title: "Storage configuration"
permalink: en/storage/admin/supported-storage.html
---

Storage setup consists of several steps that depend on the selected storage type. The main configuration stages include:

- Enabling and configuring the appropriate modules;
- Creating Volume Groups;
- Preparing and creating StorageClass objects, followed by their assignment and usage.

Each storage type may have its own specific requirements and configuration nuances, which are described in the corresponding sections.

To create StorageClass objects, you must connect one or more storages that manage PersistentVolume resources. Created StorageClass objects can be used to organize virtual disks and images.

## How to Set the Default StorageClass?

The default StorageClass is used when a PersistentVolumeClaim resource is created without explicitly specifying a storage class. This simplifies the process of creating and using storage by avoiding the need to specify a class each time.

To set the default StorageClass, specify the required storage class in the global configuration. Example command:

```shell
# Specify the name of your StorageClass object.
DEFAULT_STORAGE_CLASS=replicated-storage-class
d8 k patch mc global --type='json' -p='[{"op": "replace", "path": "/spec/settings/defaultClusterStorageClass", "value": "'"$DEFAULT_STORAGE_CLASS"'"}]'
```
