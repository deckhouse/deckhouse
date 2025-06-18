---
title: "Cloud provider — DVP"
---

The `cloud-provider-dvp` module is responsible for interacting with the [DVP](https://deckhouse.io/products/virtualization-platform/) cloud resources. It allows the [`node-manager`](../../modules/040-node-manager/) module to use DVP resources for provisioning nodes for the specified [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).

Key features of the `cloud-provider-dvp` module:

- Managing DVP resources using the `cloud-controller-manager` module.
- Provisioning disks using the `CSI storage` component.
- Integrating with the [`node-manager`](../../modules/040-node-manager/) module so that [DVPInstanceClasses](cr.html#dvpinstanceclass) can be used when defining a [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).
