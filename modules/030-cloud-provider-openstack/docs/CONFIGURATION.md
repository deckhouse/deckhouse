---
title: "Сloud provider — OpenStack: configuration"
---

The module is automatically enabled for all cloud clusters deployed in OpenStack.

You can configure the number and parameters of ordering machines in the cloud via the [`NodeGroup`](/modules/040-node-manager/cr.html#nodegroup) custom resource of the node-manager module. Also, in this custom resource, you can specify the instance class's name for the above group of nodes (the `cloudInstances.ClassReference` NodeGroup parameter). In the case of the OpenStack-based cloud provider, the instance class is the [`OpenStackInstanceClass`](cr.html#openstackinstanceclass) custom resource that stores specific parameters of the machines.

## Parameters

The module settings are set automatically based on the placement strategy chosen. In most cases, you do not have to configure the module manually.

If you need to configure a module because, say, you have a bare-metal cluster and you need to enable additional instances from vSphere, then refer to the [How to configure a Hybrid cluster in vSphere](faq.html#how-do-i-create-a-hybrid-cluster) section.

If you have instances in the cluster that use External Networks (other than those set out in the placement strategy), you must pass them via the:

* `additionalExternalNetworkNames` — parameter. It specifies additional networks that can be connected to the VM. `cloud-controller-manager` uses them to insert `ExternalIP` to `.status.addresses` field in the Node API object;
  * Format — an array of strings;

### An example

```yaml
cloudProviderOpenstack: |
  additionalExternalNetworkNames:
  - some-bgp-network
```

## Storage

The module automatically creates StorageClasses that are available in OpenStack. Also, it can filter out the unnecessary StorageClasses (you can do this via the `exclude` parameter).

* `exclude` — a list of StorageClass names (or regex expressions for names) to exclude from the creation in the cluster;
  * Format — an array of strings;
  * An optional parameter;
* `default` — the name of StorageClass that will be used in the cluster by default;
  * Format — a string;
  * An optional parameter;
  * If the parameter is omitted, the default StorageClass is either: 
    * an arbitrary StorageClass present in the cluster that has the default annotation;
    * the first StorageClass created by the module (in accordance with the order in OpenStack).

```yaml
cloudProviderOpenstack: |
  storageClass:
    exclude:
    - .*-hdd
    - iscsi-fast
    default: ceph-ssd
```
