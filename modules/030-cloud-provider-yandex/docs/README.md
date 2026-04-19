---
title: "Cloud provider — Yandex Cloud"
description: "Cloud resource management in Deckhouse Kubernetes Platform using Yandex Cloud."
---

The `cloud-provider-yandex` module is responsible for interacting with the [Yandex Cloud](https://cloud.yandex.com/en/) cloud resources. It allows the node manager module to use Yandex Cloud resources for provisioning nodes for the defined [node group](../../modules/node-manager/cr.html#nodegroup) (a group of nodes that are acted upon as if they were a single entity).

Features of the `cloud-provider-yandex` module:

- Managing Yandex Cloud resources using the `cloud-controller-manager` (CCM) module:
  - Creating network routes for the `PodNetwork` network on the Yandex Cloud side.
  - Updating Yandex Cloud Instances and Kubernetes Nodes metadata. Deleting nodes from Kubernetes that are no longer in Yandex Cloud.
- Provisioning disks in Yandex Cloud using the `CSI storage` component.
- Registration in the [node-manager](../../modules/node-manager/) module, so that [YandexInstanceClasses](cr.html#yandexinstanceclass) can be used when creating the [NodeGroup](../../modules/node-manager/cr.html#nodegroup).
- Enabling the necessary CNI plugin (which uses [simple bridge](../../modules/cni-simple-bridge/)).
