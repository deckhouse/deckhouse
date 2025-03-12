---
title: "Storage configuration"
permalink: en/storage/admin/supported-storage.html
---

Storage configuration is carried out in several steps, depending on the selected storage type. The main stages include enabling and configuring the corresponding modules, creating volume groups (Volume Groups), preparing and creating StorageClass objects, as well as their subsequent assignment and use. It is important to note that each storage type may have its own specific requirements and configuration nuances, which are described in the relevant sections.

To create StorageClass objects, you must connect one or more storages that manage PersistentVolume resources. Created StorageClass objects can be used to organize virtual disks and images.

## How to Set the Default StorageClass?

The default StorageClass is used when a PersistentVolumeClaim resource is created without explicitly specifying a storage class. This simplifies the process of creating and using storage by avoiding the need to specify a class each time.

To set the default StorageClass, specify the required storage class in the global configuration. Example command:

```shell
# Specify the name of your StorageClass object.
DEFAULT_STORAGE_CLASS=replicated-storage-class
d8 k patch mc global --type='json' -p='[{"op": "replace", "path": "/spec/settings/defaultClusterStorageClass", "value": "'"$DEFAULT_STORAGE_CLASS"'"}]'
```
