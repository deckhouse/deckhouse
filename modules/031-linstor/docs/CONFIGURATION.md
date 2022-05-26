---
title: "The linstor module: configuration"
---

This module is **disabled** by default. To enable it, add the following lines to the `deckhouse` ConfigMap:
```yaml
data:
  linstorEnabled: "true"
```

The module requires no configuration and has no parameters.

After enabling the module, the cluster is automatically configured to use LINSTOR, and all that remains is to configure the storage.

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
     lvcreate -L 1.8T -T data_project/thindata --add-tag linstor-thindata
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

## Additional features for configuring applications  

### Placing the application "closer" to the data (data locality)

In a hyperconverged infrastructure you may want your Pods to run on the same nodes as their data volumes, as it can help get the best performance from the storage.

The linstor module provides a custom kube-scheduler `linstor` for such tasks, that takes into account the placement of data in storage and tries to place Pod first on those nodes where data is available locally.

Specify the `schedulerName: linstor` parameter in the Pod description to use the `linstor` scheduler.

[Example...](usage.html#using-the-linstor-scheduler)

### Application transfer to another node in case of storage problems (fencing)

In case your application does not support high availability and runs in a single instance, you may want to force a migration to another node when storage problems occur. 

To solve the problem, specify the label `linstor.csi.linbit.com/on-storage-lost: remove` in the Pod description. The linstor module will automatically remove such Pods from the node where the storage problem occurred, allowing Kubernetes to restart the application on another node. 

[Example...](usage.html#application-transfer-to-another-node-in-case-of-storage-problems-fencing)
