---
title: "The linstor module: configuration examples"
---

<div class="docs__information warning active">
The module is actively developed, and it might significantly change in the future.
</div>

LINSTOR supports several different storage backends based on well-known and proven technologies such as LVM and ZFS.  
Each volume created in LINSTOR will be placed on one or more nodes of your cluster and replicated using DRBD.

We're working hard to make Deckhouse easy to use and don't want to overwhelm you with too much information. Therefore, we decided to provide a simple and familiar interface for configuring LINSTOR. 
After enabling the module, your cluster will be automatically configured. All what you need is to create so-called storage pools.

We currently support two modes: **LVM** and **LVMThin**.
Each of them has its advantages and disadvantages, see [FAQ](faq.html) for more details and comparison. 

LINSTOR in Deckhouse can be configured by assigning special tag `linstor-<pool_name>` to an LVM volume group or thin pool.  
Tags must be unique within the same node. Therefore, each time before assigning a new tag, make sure that other volume groups and thin pools do not have this tag already:
```
vgs -o name,tags
lvs -o name,vg_name,tags
```


* **LVM**

   To add an LVM pool, create a volume group with the `linstor-<pool_name>` tag, example:

   ```
   vgcreate linstor_data /dev/nvme0n1 /dev/nvme1n1 --add-tag linstor-data
   ```

   You can also add an existing volume group to LINSTOR:

   ```
   vgchange vg0 --add-tag linstor-data
   ```

* **LVMThin**

   To add LVMThin pool, create a LVM thin pool with the `linstor-<pool_name> tag, example:

   `` `
   vgcreate linstor_data /dev/nvme0n1 /dev/nvme1n1
   lvcreate -L 1.8T -T linstor_data/data --add-tag linstor-data
   ```

   (Take attention: the group itself should not have this tag configured)

Using the commands above, create storage pools on all nodes where you plan to store your data.  
Use the same names for storage pools on different nodes if you want them to fall into the same StorageClass.

When all storage pools are created, you will see several new StorageClasses in Kubernetes: 
```console
$ kubectl get storageclass
NAME                   PROVISIONER                  AGE
linstor-data-r1        linstor.csi.linbit.com       143s
linstor-data-r2        linstor.csi.linbit.com       142s
linstor-data-r3        linstor.csi.linbit.com       142s
```

Each of them can be used to create volumes with 1, 2 or 3 replicas in your storage pools.

You can always refer to [Advanced LINSTOR Configuration](advanced_usage.html) if needed, but we strongly recommend sticking to this simplified guide. 
