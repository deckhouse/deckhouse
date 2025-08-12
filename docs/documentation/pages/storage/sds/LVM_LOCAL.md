---
title: "Local storage based on LVM"
permalink: en/storage/admin/sds/lvm-local.html
---

{% alert level="info" %}
<span style="border-bottom: 1px dotted #000;" data-tippy-content="Restriction on the ability to create snapshots">
Available with limitations in:</span>  **CE**

Available without limitations in the following commercial editions:  **SE, SE+, EE**
{% endalert %}

Using local storage helps avoid network latencies, which improves performance compared to remote storage that requires network connectivity. This approach is ideal for test environments and EDGE clusters.

## Enabling the module

Configuring local block storage is based on the Logical Volume Manager (LVM). LVM is managed by the `sds-node-configurator` module, which must be enabled before activating the `sds-local-volume` module.

To enable the module, apply the following `ModuleConfig` resource:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: sds-node-configurator
spec:
  enabled: true
  version: 1
EOF
```

Wait until the `sds-node-configurator` module transitions to the `Ready` state. To check its status, execute the following command:

```shell
d8 k get modules sds-node-configurator -w
```

In the output, you should see information about the `sds-node-configurator` module:

```console
NAME                    STAGE   SOURCE      PHASE       ENABLED   READY
sds-node-configurator           deckhouse   Available   False     False
```

Then, to enable the `sds-local-volume` module with default settings, run the following command:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: sds-local-volume
spec:
  enabled: true
  version: 1
EOF
```

This will launch service pods of the `sds-local-volume` components on all cluster nodes. To check the module status, run the following command:

```shell
d8 k get modules sds-local-volume -w
```

In the output, you should see information about the state of the `sds-local-volume` module:

```console
NAME                    STAGE   SOURCE    PHASE       ENABLED   READY
sds-local-volume                          Available   True      True
```

To verify that all pods in the `d8-sds-local-volume` and `d8-sds-node-configurator` namespaces are in the `Running` or `Completed` state and are deployed on all nodes where LVM resources are planned to be used, use the following commands:

```shell
d8 k -n d8-sds-local-volume get pod -w
d8 k -n d8-sds-node-configurator get pod -w
```

## Node preconfiguration

### Creating LVM volume groups

Ensure that on all nodes where LVM resources are intended to be used, service pods `sds-local-volume-csi-node` are running. These pods provide interaction with nodes where LVM components are located. This can be verified using the following command:

```shell
d8 k -n d8-sds-local-volume get pod -l app=sds-local-volume-csi-node -owide
```

The placement of these pods on nodes is determined based on specific labels (nodeSelector) defined in the `spec.settings.dataNodes.nodeSelector` field in the module settings. For more details on configuration, refer to the documentation.

Before setting up the creation of StorageClass objects, available block devices on nodes need to be combined into LVM volume groups. These volume groups will subsequently be used to host PersistentVolume resources.

To list available block devices, you can use the BlockDevices resource, which reflects their current state:

```shell
d8 k get bd
```

In the output, you should see a list of available block devices:

```console
NAME                                           NODE       CONSUMABLE   SIZE           PATH
dev-ef4fb06b63d2c05fb6ee83008b55e486aa1161aa   worker-0   false        976762584Ki    /dev/nvme1n1
dev-0cfc0d07f353598e329d34f3821bed992c1ffbcd   worker-0   false        894006140416   /dev/nvme0n1p6
dev-7e4df1ddf2a1b05a79f9481cdf56d29891a9f9d0   worker-1   false        976762584Ki    /dev/nvme1n1
dev-b103062f879a2349a9c5f054e0366594568de68d   worker-1   false        894006140416   /dev/nvme0n1p6
dev-53d904f18b912187ac82de29af06a34d9ae23199   worker-2   false        976762584Ki    /dev/nvme1n1
dev-6c5abbd549100834c6b1668c8f89fb97872ee2b1   worker-2   false        894006140416   /dev/nvme0n1p6
```

In the example above, six block devices are available across three nodes. To group block devices on one node, you need to create an LVM volume group using the [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/) resource.

To create an [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/) resource on node worker-0, apply the following resource, replacing the node and block device names with your own:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: LVMVolumeGroup
metadata:
  name: "vg-on-worker-0"
spec:
  type: Local
  local:
    # Replace with the name of your node for which the volume group is being created.
    nodeName: "worker-0"
  blockDeviceSelector:
    matchExpressions:
      - key: kubernetes.io/metadata.name
        operator: In
        values:
          # Replace with the names of your node's block devices for which the volume group is being created.
          - dev-ef4fb06b63d2c05fb6ee83008b55e486aa1161aa
          - dev-0cfc0d07f353598e329d34f3821bed992c1ffbcd
  # Name of the LVM volume group that will be created from the specified block devices on the selected node.
  actualVGNameOnTheNode: "vg"
  # Uncomment if thin provisioning is required. Details will be discussed later.
  # thinPools:
  #   - name: thin-pool-0
  #     size: 70%
EOF
```

Wait until the created [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/) resource transitions to the `Ready` phase. To check the resource phase, run the following command:

```shell
d8 k get lvg vg-on-worker-0 -w
```

In the output, you should see information about the resource phase:

```console
NAME             THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE       ALLOCATED SIZE   VG   AGE
vg-on-worker-0   1/1         True                    Ready   worker-0   360484Mi   30064Mi          vg   1h
```

If the resource transitions to the `Ready` phase, this indicates that an LVM volume group named `vg` has been created on node worker-0 using the block devices `/dev/nvme1n1` and `/dev/nvme0n1p6`.

Next, you need to repeat the creation of [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/) resources for the remaining nodes (worker-1 and worker-2), modifying the resource name, node name, and block device names accordingly. Ensure that LVM volume groups are created on all nodes where they are intended for use by running the following command:

```shell
d8 k get lvg -w
```

In the output, you should see a list of created volume groups:

```console
NAME             THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE       ALLOCATED SIZE   VG   AGE
vg-on-worker-0   0/0         True                    Ready   worker-0   360484Mi   30064Mi          vg   1h
vg-on-worker-1   0/0         True                    Ready   worker-1   360484Mi   30064Mi          vg   1h
vg-on-worker-2   0/0         True                    Ready   worker-2   360484Mi   30064Mi          vg   1h
```

### Creating a thick type StorageClass

The creation of StorageClass objects is done through the [LocalStorageClass](../../../reference/cr/localstorageclass/) resource, which defines the configuration for the desired storage class. Manually creating a StorageClass without a [LocalStorageClass](../../../reference/cr/localstorageclass/) can result in errors.

When creating a [LocalStorageClass](../../../reference/cr/localstorageclass/), it's crucial to select the storage type, which can be either thick or thin.

Thick pools offer high performance comparable to the storage device itself but do not support snapshot.

Example of creating a [LocalStorageClass](../../../reference/cr/localstorageclass/) resource with a thick type:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: LocalStorageClass
metadata:
  name: local-storage-class-thick
spec:
  lvm:
    lvmVolumeGroups:
      - name: vg-on-worker-0
      - name: vg-on-worker-1
      - name: vg-on-worker-2
    type: Thick
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
EOF
```

Check that the created [LocalStorageClass](../../../reference/cr/localstorageclass/) has transitioned to the `Created` phase by running the following command:

```shell
d8 k get lsc local-storage-class -w
```

In the output, you should see information about the created [LocalStorageClass](../../../reference/cr/localstorageclass/):

```console
NAME                        PHASE     AGE
local-storage-class-thick   Created   1h
```

Check that the corresponding StorageClass has been generated by running the following command:

```shell
d8 k get sc local-storage-class
```

In the output, you should see information about the generated StorageClass:

```console
NAME                        PROVISIONER                      RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
local-storage-class-thick   local.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   1h
```

### Creating a thin type StorageClass

Thin pools allow using snapshots and overprovisioning (resource over-allocation) at the cost of reduced performance.

{% alert level="warning" %}
Overprovisioning should be used with caution, ensuring free space in the pool is carefully monitored (the cluster monitoring system generates events when free space falls to 20%, 10%, 5%, and 1%). A lack of free space in the pool can lead to degradation of the module's operation and poses a real risk of data loss.
{% endalert %}

The previously created [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/) resources are suitable for creating thick storage. If you require the ability to create thin storage, update the configuration of the [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/) resources by adding a definition for a thin pool:

```yaml
d8 k patch lvg vg-on-worker-0 --type='json' -p='[
  {
    "op": "add",
    "path": "/spec/thinPools",
    "value": [
      {
        "name": "thin-pool-0",
        "size": "70%"
      }
    ]
  }
]'
```

In the updated version of the [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/), 70% of the available space will be allocated for creating thin storage. The remaining 30% can be used for thick storage.

Repeat the addition of thin pools for the remaining nodes (worker-1 and worker-2). Example of creating a [LocalStorageClass](../../../reference/cr/localstorageclass/) resource with a thin type:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: LocalStorageClass
metadata:
  name: local-storage-class-thin
spec:
  lvm:
    lvmVolumeGroups:
      - name: vg-on-worker-0
        thin:
          - name: thin-pool-0
      - name: vg-on-worker-1
        thin:
          - name: thin-pool-0
      - name: vg-on-worker-2
        thin:
          - name: thin-pool-0
    type: Thin
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
EOF
```

Check that the created [LocalStorageClass](../../../reference/cr/localstorageclass/) has transitioned to the `Created` phase by running the following command:

```shell
d8 k get lsc local-storage-class -w
```

In the output, you should see information about the created [LocalStorageClass](../../../reference/cr/localstorageclass/):

```console
NAME                       PHASE     AGE
local-storage-class-thin   Created   1h
```

Check that the corresponding StorageClass has been generated by running the following command:

```shell
d8 k get sc local-storage-class
```

In the output, you should see information about the generated StorageClass:

```console
NAME                       PROVISIONER                      RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
local-storage-class-thin   local.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   1h
```
