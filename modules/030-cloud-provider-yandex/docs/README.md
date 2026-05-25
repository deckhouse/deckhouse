---
title: "Cloud provider — Yandex Cloud"
description: "Cloud resource management in Deckhouse Kubernetes Platform using Yandex Cloud."
---

The `cloud-provider-yandex` module is responsible for interacting with the [Yandex Cloud](https://cloud.yandex.com/en/) cloud resources. It allows the node manager module to use Yandex Cloud resources for provisioning nodes for the defined [node group](/modules/node-manager/cr.html#nodegroup) (a group of nodes that are acted upon as if they were a single entity).

Features of the `cloud-provider-yandex` module:

- Managing Yandex Cloud resources using the `cloud-controller-manager` (CCM) module:
  - Creating network routes for the `PodNetwork` network on the Yandex Cloud side.
  - Updating Yandex Cloud Instances and Kubernetes Nodes metadata. Deleting nodes from Kubernetes that are no longer in Yandex Cloud.
- Provisioning disks in Yandex Cloud using the `CSI storage` component.
- Registration in the [node-manager](/modules/node-manager/) module, so that [YandexInstanceClasses](cr.html#yandexinstanceclass) can be used when creating the [NodeGroup](/modules/node-manager/cr.html#nodegroup).
- Enabling the necessary CNI plugin (which uses [`cni-cilium`](/modules/cni-cilium/)).

{% alert level="warning" %}
Starting with DKP version 1.76, Yandex Cloud uses the `cilium` CNI by default for new clusters. Existing clusters keep the current CNI configuration.

New clusters require Linux kernel version 5.8 or newer on all nodes. Make sure firewalls or security groups allow inter-node UDP traffic for Cilium VXLAN. For details, see the [installation requirements](/products/kubernetes-platform/documentation/v1/installing/), [Network interaction of the platform components](/products/kubernetes-platform/documentation/v1/reference/network_interaction.html), and the [`cni-cilium` module documentation](/modules/cni-cilium/).
{% endalert %}
