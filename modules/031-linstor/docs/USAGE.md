---
title: "The linstor module: configuration examples"
---

LINSTOR supports several different storage backends based on well-known and proven technologies such as LVM and ZFS.  
Each volume created in LINSTOR will be placed on one or more nodes of your cluster and replicated using DRBD.

We're working hard to make Deckhouse easy to use and don't want to overwhelm you with too much information. Therefore, we decided to provide a simple and familiar interface for configuring LINSTOR. 
After enabling the module, your cluster will be automatically configured. All what you need is to create so-called storage pools.

We currently support two modes: **LVM** and **LVMThin**.

Each of them has its advantages and disadvantages, read [FAQ](faq.html) for more details and comparison. 

LINSTOR in Deckhouse can be configured by assigning special tag `linstor-<pool_name>` to an LVM volume group or thin pool.  
Tags must be unique within the same node. Therefore, before assigning a new tag, make sure that other volume groups and thin pools do not have this tag already by executing the following commands:
```shell
vgs -o name,tags
lvs -o name,vg_name,tags
```

* **LVM**

   To add an LVM pool, create a volume group with the `linstor-<pool_name>` tag, e.g., using the following command:

   ```shell
   vgcreate linstor_data /dev/nvme0n1 /dev/nvme1n1 --add-tag linstor-data
   ```

   You can also add an existing volume group to LINSTOR, e.g., using the following command:

   ```shell
   vgchange vg0 --add-tag linstor-data
   ```

* **LVMThin**

   To add LVMThin pool, create a LVM thin pool with the `linstor-<pool_name> tag, e.g., using the following command:

   ```shell
   vgcreate linstor_data /dev/nvme0n1 /dev/nvme1n1
   lvcreate -L 1.8T -T linstor_data/thindata --add-tag linstor-thindata
   ```

   (Take attention: the group itself should not have this tag configured)

Using the commands above, create storage pools on all nodes where you plan to store your data.  
Use the same names for the storage pools on the different nodes if you want to achieve a general StorageClasses created for all of them.

When all storage pools are created, you will see up to three new StorageClasses in Kubernetes:
```console
$ kubectl get storageclass
NAME                   PROVISIONER                  AGE
linstor-data-r1        linstor.csi.linbit.com       143s
linstor-data-r2        linstor.csi.linbit.com       142s
linstor-data-r3        linstor.csi.linbit.com       142s
```

Each of them can be used to create volumes with 1, 2 or 3 replicas in your storage pools.

You can always refer to [Advanced LINSTOR Configuration](advanced_usage.html) if needed, but we strongly recommend sticking to this simplified guide. 

## Data Locality

In a hyperconverged infrastructure you may want your Pods to run on the same nodes as their data volumes. The **linstor** module provides a custom kube-scheduler for such tasks.

Create a Pod with `schedulerName: linstor` in order to prioritize placing the Pod "closer to the data" and get the best performance from the disk subsystem.

An example of such a Pod:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: busybox
  namespace: default
spec:
  schedulerName: linstor
  containers:
  - name: busybox
    image: busybox
    command: ["tail", "-f", "/dev/null"]
    volumeMounts:
    - name: my-first-linstor-volume
      mountPath: /data
    ports:
    - containerPort: 80
  volumes:
  - name: my-first-linstor-volume
    persistentVolumeClaim:
      claimName: "test-volume"
```

## Fencing

In case your application does not support high availability, you can add a special annotation that will allow **linstor** to automatically remove your application's Pod from the failed node. This will allow Kubernetes to safely restart your application on the new node.

Example StatefulSet with this annotation:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: my-stateful-app
spec:
  serviceName: my-stateful-app
  selector:
    matchLabels:
      app.kubernetes.io/name: my-stateful-app
  template:
    metadata:
      labels:
        app.kubernetes.io/name: my-stateful-app
        linstor.csi.linbit.com/on-storage-lost: remove
    ...
```
