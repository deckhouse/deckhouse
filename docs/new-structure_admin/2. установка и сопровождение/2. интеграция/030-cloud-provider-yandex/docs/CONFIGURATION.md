---
title: "Cloud provider — Yandex Cloud: configuration"
---

> **Note!** If the parameters provided below are changed, the **existing Machines are NOT redeployed** (new machines will be created with the updated parameters). Redeployment is only performed when `NodeGroup` and `YandexInstanceClass` are changed. Details in the [node-manager module's documentation](../../modules/040-node-manager/faq.html#how-do-i-redeploy-ephemeral-machines-in-the-cloud-with-a-new-configuration).

{% include module-settings.liquid %}

## Storage

The module automatically creates StorageClasses covering all available disks in Yandex:

| Type | StorageClass Name | Comment |
|---|---|---|
| network-hdd | network-hdd | |
| network-ssd | network-ssd | |
| network-ssd-nonreplicated | network-ssd-nonreplicated | |
| network-ssd-io-m3         | network-ssd-io-m3 | Disk size must be a multiple of 93 GB. |

You can filter out the unnecessary StorageClasses via the [exclude](#parameters-storageclass-exclude) parameter.

### Important information concerning the increase of the PVC size

Due to the [nature](https://github.com/kubernetes-csi/external-resizer/issues/44) of volume-resizer, CSI, and Yandex Cloud API, you have to do the following after increasing the PVC size:

1. On the node where the Pod is located, run the `kubectl cordon <node_name>` command.
2. Delete the Pod.
3. Make sure that the resize was successful. The PVC object *must not have* the `Resizing` state.
   > The `FileSystemResizePending` state is OK.
4. On the node where the Pod is located, run the `kubectl uncordon <node_name>` command.

## LoadBalancer

The module subscribes to Service objects of the `LoadBalancer` type and creates the corresponding `NetworkLoadBalancer` and `TargetGroup` in Yandex Cloud.

For more information, see the [Kubernetes Cloud Controller Manager for Yandex Cloud documentation](https://github.com/flant/yandex-cloud-controller-manager).
