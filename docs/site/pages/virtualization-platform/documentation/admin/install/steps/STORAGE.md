---
title: "Set up storage"
permalink: en/virtualization-platform/documentation/admin/install/steps/storage.html
---

After adding worker nodes, it is necessary to configure the storage that will be used for creating virtual machine disks and storing cluster component metrics. The storage can be selected from the [supported list](/products/virtualization-platform/documentation/about/requirements.html#supported-storage-systems).

Next, we will consider using software-defined replicated block storage based on DRBD, which allows creating replicated volumes based on the disk space of nodes. As an example, we will configure a StorageClass based on volumes with two replicas, located on the disks `/dev/sda`.

{% alert level=“info” %}
To run the commands below, you need to have the [d8 utility](/products/kubernetes-platform/documentation/v1/cli/d8/) (Deckhouse CLI) installed and a configured kubectl context for accessing the cluster. Alternatively, you can connect to the master node via SSH and run the command as the `root` user using `sudo -i`.
{% endalert %}

## Enabling the use of replicated storage

Enable the `sds-node-configurator`, `sds-replicated-volume`, and `snapshot-controller` modules using either the administrator web interface or the CLI:

1. Enable the `sds-node-configurator` module:

   ```shell
   sudo -i d8 system module enable sds-node-configurator
   ```

1. Wait until the `sds-node-configurator` module reaches the `Ready` state:

   ```shell
   d8 k get module sds-node-configurator -w
   ```

1. Enable the `snapshot-controller` module:

   ```shell
   sudo -i d8 system module enable snapshot-controller
   ```

1. Enable the `sds-replicated-volume` module:

   ```shell
   sudo -i d8 system module enable sds-replicated-volume
   ```

1. Wait until the `sds-replicated-volume` module reaches the `Ready` state:

   ```shell
   sudo -i d8 k wait module sds-replicated-volume --for='jsonpath={.status.status}=Ready' --timeout=1200s
   ```

1. Verify that in the `d8-sds-node-configurator`, `d8-snapshot-controller`, and `d8-sds-replicated-volume` namespaces, all pods are in the `Running` or `Completed` status:

   ```shell
   sudo -i d8 k -n d8-sds-replicated-volume get pod -owide -w
   sudo -i d8 k -n d8-sds-node-configurator get pod -owide -w
   sudo -i d8 k -n d8-sds-node-configurator get pod -owide -w
   ```

## Configuration of replicated storage

Configuring the storage involves combining the available block devices on the nodes into pools, from which a StorageClass will then be created.

1. Retrieve the available block devices:

   ```shell
   d8 k get blockdevices.storage.deckhouse.io
   ```

   {% offtopic title="Example output with additional sda disks..." %}

   ```console
   NAME                                           NODE           CONSUMABLE   SIZE          PATH        AGE
   dev-93640bc74158c6e491a2f257b5e0177309588db0   master-0       false        468851544Ki   /dev/sda    8m28s
   dev-40bf7a561aee502f20b81cf1eff873a0455a95cb   dvp-worker-1   false        468851544Ki   /dev/sda    8m17s
   dev-b1c720a7cec32ae4361de78b71f08da1965b1d0c   dvp-worker-2   false        468851544Ki   /dev/sda    8m12s
   ```

   {% endofftopic %}

1. Create a VolumeGroup on each node.

   On each node, you need to create an LVM volume group using the [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resource.

   To create the LVMVolumeGroup resource, use the following commands on each node (specify the node name and block device name):

   ```shell
   export NODE_NAME="<NODE_NAME>"
   export DEV_NAME="<BLOCK_DEVICE_NAME>"
   d8 k apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: LVMVolumeGroup
   metadata:
     name: "vg-on-${NODE_NAME}"
   spec:
     type: Local
     local:
       nodeName: "$NODE_NAME"
     blockDeviceSelector:
       matchExpressions:
         - key: kubernetes.io/metadata.name
           operator: In
           values:
             - "$DEV_NAME"
     # The name of the LVM volume group that will be created from the block devices on the selected node.
     actualVGNameOnTheNode: "vg-1"
   EOF
   ```

   Wait for all the created LVMVolumeGroup resources to transition to the `Ready` state:

   ```shell
   d8 k get lvg -w
   ```

   {% offtopic title="Example output..." %}

   ```console
   NAME                THINPOOLS  CONFIGURATION APPLIED   PHASE   NODE          SIZE       ALLOCATED SIZE VG   AGE
   vg-on-master-0      0/0        True                    Ready   master-0      360484Mi   30064Mi        vg-1 29s
   vg-on-dvp-worker-1  0/0        True                    Ready   dvp-worker-1  360484Mi   30064Mi        vg-1 58s
   vg-on-dvp-worker-2  0/0        True                    Ready   dvp-worker-2  360484Mi   30064Mi        vg-1 6s
   ```

   {% endofftopic %}

1. Create a pool of LVM volume groups.

   Created volume groups need to be assembled into a pool for replication (specifies in ReplicatedStoragePool resource). To do this, run the following command (specify the names of the created volume groups):

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStoragePool
   metadata:
     name: sds-pool
   spec:
     type: LVM
     lvmVolumeGroups:
       # Укажите свои имена групп томов.
       - name: vg-on-dvp-worker-01
       - name: vg-on-dvp-worker-02
       - name: vg-on-master
   EOF
   ```

   Wait for the resource to transition to the `Completed` state:

   ```shell
   d8 k get rsp data -w
   ```

   {% offtopic title="Example output..." %}

   ```console
   NAME         PHASE       TYPE   AGE
   sds-pool     Completed   LVM    32s
   ```

   {% endofftopic %}

1. Set StorageClass parameters.

   The `sds-replicated-volume` module uses the ReplicatedStorageClass resources to automatically create StorageClasses with the required characteristics. The following parameters are important in this resource:

   - `replication`: Replication parameters, for 2 replicas, the value `Availability` will be used.
   - `storagePool`: The name of the pool created earlier, in this example, it is `sds-pool`.

   Other parameters are described in the [ReplicatedStorageClass resource documentation](/modules/sds-replicated-volume/cr.html#replicatedstorageclassreplicatedstorageclass.html).

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: sds-r2
   spec:
     replication: Availability
     storagePool: sds-pool
     reclaimPolicy: Delete
     topology: Ignored
   EOF
   ```

   Check that the corresponding StorageClass has appeared in the cluster:

   ```shell
   d8 k get sc
   ```

   {% offtopic title="Example output..." %}

   ```console
   NAME     PROVISIONER                           RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
   sds-r2   replicated.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   6s
   ```

   {% endofftopic %}

1. Set the default StorageClass (specify the name of your StorageClass object):

   ```shell
   DEFAULT_STORAGE_CLASS=<DEFAULT_STORAGE_CLASS_NAME>
   sudo -i d8 k patch mc global --type='json' -p='[{"op": "replace", "path": "/spec/settings/defaultClusterStorageClass", "value": "'"$DEFAULT_STORAGE_CLASS"'"}]'
   ```
