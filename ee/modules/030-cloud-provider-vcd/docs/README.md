---
title: "Cloud provider — VMware Cloud Director"
description: "Cloud resource management in Deckhouse Kubernetes Platform using VMware Cloud Director."
---

The `cloud-provider-vcd` module is responsible for interacting with the VMware Cloud Director resources. It allows the [node manager](../../modules/node-manager/) module to use VMware Cloud Director resources for provisioning nodes for the specified [node group](../../modules/node-manager/cr.html#nodegroup) (a group of nodes that are acted upon as if they were a single entity).
