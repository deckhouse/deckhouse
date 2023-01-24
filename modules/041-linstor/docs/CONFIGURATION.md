---
title: "The linstor module: configuration"
---

{% include module-bundle.liquid %}

> The functionality of the module is guaranteed only for stock kernels for distributions listed in the [list of supported OS](../../supported_versions.html#linux).
> The functionality of the module with other kernels is possible but not guaranteed.

After enabling the module, the cluster is automatically configured to use LINSTOR, and all that remains is to configure the storage.

The module requires no configuration and has no parameters. However, some functions may require a master passphrase.  
To set a master passphrase, create a Secret in the `d8-system` namespace:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: linstor-passphrase
  namespace: d8-system
immutable: true
stringData:
  MASTER_PASSPHRASE: *!passphrase* # Master passphrase for LINSTOR
```

> **Warning**: Choose strong passphrase and store it securely. If it get lost, the encrypted data will be inaccessible.

## LINSTOR storage configuration

LINSTOR in Deckhouse can be configured by assigning special tag `linstor-<pool_name>` to an LVM volume group or LVMThin pool.

1. Choose the tag name.

   The tag name must be unique within the same node. Therefore, before assigning a new tag, make sure that other volume groups and thin pools do not have this tag already.

   Execute the following commands to get list volume groups and pools:

   ```shell
   vgs -o name,tags
   lvs -o name,vg_name,tags
   ```

1. Add pools.

   Add pools on all nodes where you plan to store your data. Use the same names for the storage pools on the different nodes if you want to achieve a general StorageClasses created for all of them.

   - To add an **LVM** pool, create a volume group with the `linstor-<pool_name>` tag, or the `linstor-<pool_name>` tag to an existing volume group.

     Example of command to create a volume group `data_project` with the `linstor-data` tag :

     ```shell
     vgcreate data_project /dev/nvme0n1 /dev/nvme1n1 --add-tag linstor-data
     ```

     Example of command to add the `linstor-data` tag to an existing volume group `data_project`:

     ```shell
     vgchange data_project --add-tag linstor-data
     ```

   - To add an **LVMThin** pool, create a LVM thin pool with the `linstor-<pool_name>` tag.

     Example of command to create the LVMThin pool `data_project/thindata` with the `linstor-data` tag:

     ```shell
     vgcreate data_project /dev/nvme0n1 /dev/nvme1n1
     lvcreate -l 100%FREE -T data_project/thindata --add-tag linstor-thindata
     ```

     > Note, that the group itself should not have this tag configured.

1. Check the creation of StorageClass.

   Three new StorageClasses will appear when all the storage pools have been created. Check that they were created by running the following command in the Kubernetes cluster:

   ```shell
   kubectl get storageclass
   ```

   Example of the output:

   ```shell
   $ kubectl get storageclass
   NAME                   PROVISIONER                  AGE
   linstor-data-r1        linstor.csi.linbit.com       143s
   linstor-data-r2        linstor.csi.linbit.com       142s
   linstor-data-r3        linstor.csi.linbit.com       142s
   ```

   Each StorageClass can be used to create volumes with one, two, or three replicas in your storage pools, respectively.

You can always refer to [Advanced LINSTOR Configuration](advanced_usage.html) if needed, but we strongly recommend sticking to this simplified guide.
