---
title: "Cloud provider — Yandex.Cloud: configuration"
---

## Parameters

> **Note** that if the parameters provided below are changed (i.e., the parameters specified in the deckhouse ConfigMap), the **existing Machines are NOT redeployed** (new machines will be created with the updated parameters). Redeployment is only performed when `NodeGroup` and `YandexInstanceClass` are changed. You can learn more in the [node-manager module's documentation](../../modules/040-node-manager/faq.html#how-do-i-redeploy-ephemeral-machines-in-the-cloud-with-a-new-configuration).

* `additionalExternalNetworkIDs` — a list of Network IDs that will be considered `ExternalIP` when listing Node addresses;

## Storage

The module automatically creates StorageClasses covering all available disks in Yandex:

| Type | StorageClass Name |
|---|---|
| network-hdd | network-hdd |
| network-ssd | network-ssd |
| network-ssd-nonreplicated | network-ssd-nonreplicated |

Also, it can filter out the unnecessary StorageClasses (you can do this via the `exclude` parameter):

* `exclude` — a list of StorageClass names (or regex expressions for names) to exclude from the creation in the cluster;
* `default` — the name of StorageClass that will be used by default in the cluster; If the parameter is omitted, the default StorageClass is either:
  * An arbitrary StorageClass present in the cluster that has the default annotation;
  * The first StorageClass created by the module (in accordance with the order listed in the table above).

An example:

```yaml
cloudProviderYandex: |
  storageClass:
    exclude: 
    - .*-hdd
    default: network-ssd
```

### Important information concerning the increase of the PVC size

Due to the [nature](https://github.com/kubernetes-csi/external-resizer/issues/44) of volume-resizer, CSI, and Yandex.Cloud API, you have to do the following after increasing the PVC size:

1. Run the `kubectl cordon node_where_pod_is_hosted` command;
2. Delete the Pod;
3. Make sure that the resize was successful. The PVC object *must not have* the `Resizing` state. 
  > **Note (!)** that the `FileSystemResizePending` state is OK;
4. Run the `kubectl uncordon node_where_pod_is_hosted` command;

## LoadBalancer

The module subscribes to Service objects of the LoadBalancer type and creates the corresponding NetworkLoadBalancer and TargetGroup in Yandex.Cloud.

For more information, see the [CCM documentation](https://github.com/flant/yandex-cloud-controller-manager).
