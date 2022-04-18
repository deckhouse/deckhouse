---
title: "Cloud provider — Yandex.Cloud"
---

The `cloud-provider-yandex` module is responsible for interacting with the [Yandex.Cloud](https://cloud.yandex.com/en/) cloud resources. It allows the node manager module to use Yandex.Cloud resources for provisioning nodes for the defined [node group](../../modules/040-node-manager/cr.html#nodegroup) (a group of nodes that are acted upon as if they were a single entity).

The `cloud-provider-yandex` module:
- Manages Yandex.Cloud resources using the `cloud-controller-manager` (CCM) module:
    * The CCM module creates network routes for the `PodNetwork` network on the Yandex.Cloud side.
    * The CCM module updates the Yandex.Cloud Instances and Kubernetes Nodes metadata and deletes from Kubernetes nodes that no longer exist in Yandex.Cloud.
- Provisions disks in Yandex.Cloud using the `CSI storage` component.
- Registers with the [node-manager](../../modules/040-node-manager/) module so that [YandexInstanceClasses](cr.html#yandexinstanceclass) can be used when creating the [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).
- Enables the necessary CNI plugin (using the [simple bridge](../../modules/035-cni-simple-bridge/)).
