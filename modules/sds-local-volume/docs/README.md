---
title: "The sds-local-volume module"
description: "The sds-local-volume module: General Concepts and Principles."
moduleStatus: preview
---

The module manages local block storage based on LVM. It enables the creation of StorageClasses in Kubernetes using [LocalStorageClass](cr.html#localstorageclass) custom resources.

To create a StorageClass, you must first configure LVMVolumeGroup on the cluster nodes. The LVM configuration is handled by the [sds-node-configurator](../../sds-node-configurator/stable/) module.

## Module setup steps

To ensure the correct operation of the `sds-local-volume` module, follow these steps:

- Configure LVMVolumeGroup.

  Before creating a StorageClass, you need to create the [LVMVolumeGroup](../../sds-node-configurator/stable/cr.html#lvmvolumegroup) resource of the `sds-node-configurator` module on the cluster nodes.

- Enable the [sds-node-configurator](../../sds-node-configurator/stable/) module.

  Ensure that the `sds-node-configurator` module is enabled **before** enabling the `sds-local-volume` module.

- Create the corresponding StorageClasses.

  The creation of StorageClasses for the CSI driver `local.csi.storage.deckhouse.io` is **prohibited** for users.

The module supports two operation modes: LVM and LVMThin.  
Each mode has its own features, advantages, and limitations. For more details on the differences, refer to the [FAQ](./faq.html#when-to-use-lvm-and-when-to-use-lvmthin).

## Quickstart guide

Note that all commands must be run on a machine that has administrator access to the Kubernetes API.

### Enabling modules

Enabling the `sds-node-configurator` module:

1. Create a ModuleConfig resource to enable the module:

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: sds-node-configurator
   spec:
     enabled: true
     version: 1
   EOF
   ```

1. Wait for the module to become `Ready`. At this stage, there is no need to check the pods in the `d8-sds-node-configurator`.

   ```shell
   kubectl get modules sds-node-configurator -w
   ```

Enabling the `sds-local-volume` module:

1. Activate the `sds-local-volume` module. Before enabling it, we recommend reviewing the [available settings](./configuration.html). The example below starts the module with default settings, which will result in service pods of the `sds-local-volume` component being deployed on all cluster nodes:

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: sds-local-volume
   spec:
     enabled: true
     version: 1
   EOF
   ```

1. Wait for the module to become `Ready`.

   ```shell
   kubectl get modules sds-local-volume -w
   ```

1. Make sure that all pods in `d8-sds-local-volume` and `d8-sds-node-configurator` namespaces are `Running` or `Completed` and are running on all nodes where LVM resources are intended to be used.

   ```shell
   kubectl -n d8-sds-local-volume get pod -owide -w
   kubectl -n d8-sds-node-configurator get pod -o wide -w
   ```

### Preparing nodes for storage provisioning

To create storage on nodes, it's necessary for the `sds-local-volume-csi-node` pods to be running on the selected nodes.

By default, the pods will be scheduled on all nodes in the cluster. You can verify their presence using the command:

```shell
kubectl -n d8-sds-local-volume get pod -owide
```

The location of the pod data is determined by special labels (nodeSelector) specified in the [spec.settings.dataNodes.nodeSelector](configuration.html#parameters-datanodes-nodeselector) field in the module settings. Read more in the [module FAQ](./faq.html#i-dont-want-the-module-to-be-used-on-all-nodes-of-the-cluster-how-can-i-select-the-desired-nodes).

### Configuring storage on nodes

You need to create LVM volume groups on the nodes using LVMVolumeGroup custom resources. As part of this quickstart guide, we will create a regular storage Thick.

{{< alert level="warning" >}}
Please ensure that the `sds-local-volume-csi-node` pod is running on the node before creating the `LVMVolumeGroup`. You can do this using the command:

```shell
kubectl -n d8-sds-local-volume get pod -owide
```

{{< /alert >}}

#### Storage setup steps

1. Get all available [BlockDevice](../../sds-node-configurator/stable/cr.html#blockdevice) resources available in your cluster:

   ```shell
   kubectl get bd

   NAME                                           NODE       CONSUMABLE   SIZE           PATH
   dev-ef4fb06b63d2c05fb6ee83008b55e486aa1161aa   worker-0   false        976762584Ki    /dev/nvme1n1
   dev-0cfc0d07f353598e329d34f3821bed992c1ffbcd   worker-0   false        894006140416   /dev/nvme0n1p6
   dev-7e4df1ddf2a1b05a79f9481cdf56d29891a9f9d0   worker-1   false        976762584Ki    /dev/nvme1n1
   dev-b103062f879a2349a9c5f054e0366594568de68d   worker-1   false        894006140416   /dev/nvme0n1p6
   dev-53d904f18b912187ac82de29af06a34d9ae23199   worker-2   false        976762584Ki    /dev/nvme1n1
   dev-6c5abbd549100834c6b1668c8f89fb97872ee2b1   worker-2   false        894006140416   /dev/nvme0n1p6
   ```

1. Create an [LVMVolumeGroup](../../sds-node-configurator/stable/cr.html#lvmvolumegroup) resource for `worker-0`:

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: LVMVolumeGroup
   metadata:
     name: "vg-1-on-worker-0" # The name can be any fully qualified resource name in Kubernetes. This LVMVolumeGroup resource name will be used to create LocalStorageClass in the future
   spec:
     type: Local
     local:
       nodeName: "worker-0"
     blockDeviceSelector:
       matchExpressions:
         - key: kubernetes.io/metadata.name
           operator: In
           values:
             - dev-ef4fb06b63d2c05fb6ee83008b55e486aa1161aa
             - dev-0cfc0d07f353598e329d34f3821bed992c1ffbcd
     actualVGNameOnTheNode: "vg-1" # the name of the LVM VG to be created from the above block devices on the node 
   EOF
   ```

1. Wait for the created LVMVolumeGroup resource to become `Ready`:

   ```shell
   kubectl get lvg vg-1-on-worker-0 -w
   ```

   The resource becoming `Ready` means that an LVM VG named `vg-1` made up of the `/dev/nvme1n1` and `/dev/nvme0n1p6` block devices has been created on the `worker-0` node.

1. Create an [LVMVolumeGroup](../../sds-node-configurator/stable/cr.html#lvmvolumegroup) resource for `worker-1`:

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: LVMVolumeGroup
   metadata:
     name: "vg-1-on-worker-1"
   spec:
     type: Local
     local:
       nodeName: "worker-1"
     blockDeviceSelector:
       matchExpressions:
         - key: kubernetes.io/metadata.name
           operator: In
           values:
             - dev-7e4df1ddf2a1b05a79f9481cdf56d29891a9f9d0
             - dev-b103062f879a2349a9c5f054e0366594568de68d
     actualVGNameOnTheNode: "vg-1"
   EOF
   ```

1. Wait for the created LVMVolumeGroup resource to become `Ready`:

   ```shell
   kubectl get lvg vg-1-on-worker-1 -w
   ```

   The resource becoming `Ready` means that an LVM VG named `vg-1` made up of the `/dev/nvme1n1` and `/dev/nvme0n1p6` block device has been created on the `worker-1` node.

1. Create an [LVMVolumeGroup](../../sds-node-configurator/stable/cr.html#lvmvolumegroup) resource for `worker-2`:

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: LVMVolumeGroup
   metadata:
     name: "vg-1-on-worker-2"
   spec:
     type: Local
     local:
       nodeName: "worker-2"
     blockDeviceSelector:
       matchExpressions:
         - key: kubernetes.io/metadata.name
           operator: In
           values:
             - dev-53d904f18b912187ac82de29af06a34d9ae23199
             - dev-6c5abbd549100834c6b1668c8f89fb97872ee2b1
     actualVGNameOnTheNode: "vg-1"
   EOF
   ```

1. Wait for the created LVMVolumeGroup resource to become `Ready`:

   ```shell
   kubectl get lvg vg-1-on-worker-2 -w
   ```

   The resource becoming `Ready` means that an LVM VG named `vg-1` made up of the `/dev/nvme1n1` and `/dev/nvme0n1p6` block device has been created on the `worker-2` node.

1. Create a [LocalStorageClass](./cr.html#localstorageclass) resource:

   ```yaml
   kubectl apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: LocalStorageClass
   metadata:
     name: local-storage-class
   spec:
     lvm:
       lvmVolumeGroups:
         - name: vg-1-on-worker-0
         - name: vg-1-on-worker-1
         - name: vg-1-on-worker-2
       type: Thick
     reclaimPolicy: Delete
     volumeBindingMode: WaitForFirstConsumer
   EOF
   ```

1. Wait for the created LocalStorageClass resource to become `Created`:

   ```shell
   kubectl get lsc local-storage-class -w
   ```

1. Confirm that the corresponding StorageClass has been created:

   ```shell
   kubectl get sc local-storage-class
   ```

If StorageClass with the name `local-storage-class` is shown, then the configuration of the `sds-local-volume` module is complete. Now users can create PVCs by specifying StorageClass with the name `local-storage-class`.

## System requirements and recommendations

- Use the stock kernels provided with the [supported distributions](https://deckhouse.io/documentation/v1/supported_versions.html#linux).
- Do not use another SDS (Software defined storage) to provide disks to Deckhouse SDS.
