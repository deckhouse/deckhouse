---
title: Node management in Yandex Cloud
permalink: en/admin/integrations/public/yandex/node-management.html
lang: en
---

{% alert level="warning" %}
CloudEphemeral node management is moving from Machine Controller Manager (MCM) to Cluster API (CAPI). Existing [NodeGroups](/modules/node-manager/cr.html#nodegroup) continue to use MCM, while new ones are created with CAPI by default. For migrating existing groups, see [How to migrate node groups to Cluster API (CAPI)](../../../../faq.html#how-to-migrate-node-groups-to-cluster-api-capi).
{% endalert %}

## Applying changes

Deckhouse Platform does not recreate existing Machine objects when configuration parameters change. Node recreation occurs only when:

- [NodeGroup](/modules/node-manager/cr.html#nodegroup) section parameters change.
- [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass) parameters change.

This behavior helps prevent unnecessary operations and node idling, but it means you must manually recreate VMs if needed.

If you change the [provider settings](/modules/cloud-provider-yandex/configuration.html) in the `cloud-provider-yandex` [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) (for example, the provider parameters, the [layout](../../../../glossary.html#layout), or the subnets), run the following command to apply the changes:

```shell
dhctl converge
```

This command reconciles the cluster state with the configuration defined in the resources.

## Changing networkType does not recreate CloudEphemeral nodes

In clusters that use Machine Controller Manager (MCM), changing the [`networkType`](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass-v1-spec-networktype) parameter in a YandexInstanceClass resource does not trigger automatic recreation of existing [CloudEphemeral nodes](../../../../glossary.html#cloud-ephemeral-node). Although the MachineClass is updated, the network acceleration type of existing virtual machines in Yandex Cloud remains unchanged.

Deckhouse Platform intentionally does not take the `networkType` parameter into account when determining whether CloudEphemeral nodes should be recreated in MCM. If it did, updating Deckhouse Platform would recreate CloudEphemeral nodes in all clusters where `networkType` is specified, even if its value had not changed.

In Cluster API, the `networkType` parameter is taken into account when determining whether nodes should be recreated, so changing it automatically triggers node recreation. New CloudEphemeral NodeGroups in Yandex Cloud use Cluster API by default.

If you changed `networkType` in an MCM-based cluster and want the new value to apply to existing CloudEphemeral nodes, recreate them manually — instructions in the [documentation](/modules/node-manager/faq.html#how-do-i-redeploy-ephemeral-machines-in-the-cloud-with-a-new-configuration).

## Integrating manually created VMs

Deckhouse Platform allows you to connect existing VMs in Yandex Cloud to the Kubernetes cluster as nodes. These nodes are called [CloudStatic](../../../../glossary.html#cloud-static-node) and they are not directly managed by the [`node-manager`](/modules/node-manager/) module but can still be used in the cluster.

To manually connect a VM as a CloudStatic node, follow these steps:

1. Retrieve the current `nodeNetworkCIDR` value from the cluster:

   ```shell
   d8 k get mc cloud-provider-yandex -o jsonpath='{.spec.settings.nodes.parameters.nodeNetworkCIDR}{"\n"}'
   ```

   Expected output:

   ```console
   192.168.12.0/24
   ```

   Copy this value and use it in the VM metadata as `value`.

1. Set the `node-network-cidr` parameter in the VM metadata:

   ```yaml
   key: node-network-cidr
   value: <NODE_NETWORK_CIDR>
   ```

   The `node-network-cidr` parameter must match the value of the [`nodeNetworkCIDR`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-nodenetworkcidr) parameter in the ModuleConfig.

## Replacing the OS image

The [`imageID`](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass-v1-spec-imageid) parameter of YandexInstanceClass defines the image for `CloudEphemeral` nodes. With a [ModuleConfig-based provider configuration](../../../../faq.html#how-to-migrate-a-cloud-provider-to-moduleconfig-based-configurat), this parameter also defines the image for `CloudPermanent` nodes and master nodes.

With a YandexClusterConfiguration-based configuration, the image for `CloudPermanent` nodes and master nodes is set by `imageID` in `nodeGroups[].instanceClass` and `masterNodeGroup.instanceClass`, respectively.

The replacement procedure is described in ["How do I replace the OS image in a cloud cluster?"](../../../../faq.html#how-do-i-replace-the-os-image-in-a-cloud-cluster).
