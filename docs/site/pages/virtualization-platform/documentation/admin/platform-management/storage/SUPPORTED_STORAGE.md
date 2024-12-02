---
title: "Deckhouse Virtualization Platform"
permalink: en/virtualization-platform/documentation/admin/platform-management/storage/supported-storage.html
---

To create StorageClass objects, it is necessary to connect one or more storage systems that will manage the `PersistentVolume` resources.

All supported systems can be divided into two groups:

- Software-Defined Storage (SDS), located on platform nodes.
- External storage, which can be located either on platform nodes or outside of them.

Software-Defined Storage:

- Local block storage based on LVM (Logical Volume Manager).
- Replicated block storage based on DRBD (Distributed Replicated Block Device).

External Storage:

- Ceph cluster.
- NFS (Network File System).

Created StorageClass objects can be used to organize virtual disks and images.

## How to Set a Default StorageClass?

To set a default StorageClass, specify the desired storage class in the global configuration.  
Example command:

```shell
# Specify the name of your StorageClass object.
DEFAULT_STORAGE_CLASS=replicated-storage-class
d8 k patch mc global --type='json' -p='[{"op": "replace", "path": "/spec/settings/defaultClusterStorageClass", "value": "'"$DEFAULT_STORAGE_CLASS"'"}]'
```

### Default StorageClass for virtual images

Alternatively, for virtual images with a storage type of PersistentVolumeClaim, you can set a separate default StorageClass that differs from the platform's default storage class.
You also need to explicitly specify the list of storage classes that users can select when configuring the VirtualImage resource.

To do this, edit the `ModuleConfig` for `virtualization`:

```yaml
spec:
  settings:
    virtualImages:
      # Set your default storage class.
      defaultStorageClassName: replicated-storage-class-r3
      # Define storage classes allowed for users to create virtual disks.
      allowedStorageClassSelector:
        matchNames:
        - replicated-storage-class-r1
        - replicated-storage-class-r2
        - replicated-storage-class-r3
```

### Default StorageClass for virtual disks

Alternatively, you can set a separate default StorageClass for virtual disks that differs from the platform's default storage class.  
You also need to explicitly define the list of storage classes that users can select when configuring the `VirtualDisk` resource.

To do this, edit the `ModuleConfig` for `virtualization`:

```yaml
spec:
  settings:
    virtualDisks:
      # Set your default storage class:
      defaultStorageClassName: replicated-storage-class-r3
      # Define storage classes allowed for users to create virtual disks:
      allowedStorageClassSelector:
        matchNames:
        - replicated-storage-class-r1
        - replicated-storage-class-r2
        - replicated-storage-class-r3
```

### StorageClass for the Container Registry

{% alert level="warning" %}
Changing the default storage class for the DVCR container registry will only apply if the corresponding PersistentVolumeClaim has not yet been created.
{% endalert %}

For images and disks, the DVCR container registry is used. If the DVCR container registry uses a PersistentVolumeClaim for storage,
you can explicitly define the StorageClass to be used.

To do this, edit the ModuleConfig for `virtualization`:

```yaml
spec:
  settings:
    dvcr:
      storage:
        # Use PersistentVolumeClaim as storage for the container registry.
        type: PersistentVolumeClaim
        persistentVolumeClaim:
          # Specify the name of your StorageClass.
          storageClassName: replicated-storage-class-r3
```
