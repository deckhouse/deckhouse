---
title: "Cloud provider — DVP"
---

The `cloud-provider-dvp` module is responsible for interacting with the [DVP](https://deckhouse.ru/products/virtualization-platform/) cloud resources. It allows the [node manager module](../../modules/040-node-manager/) to use DVP resources for provisioning nodes for the specified [node group](../../modules/040-node-manager/cr.html#nodegroup) (a group of nodes that are acted upon as if they were a single entity).

Key features of the `cloud-provider-dvp` module:

- Manages DVP resources using the `cloud-controller-manager` (CCM) module
- Provisions disks using the `CSI storage` component
- Registers with the [node-manager](../../modules/040-node-manager/) module so that [DVPInstanceClasses](cr.html#dvpinstanceclass) can be used when creating the [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup)
