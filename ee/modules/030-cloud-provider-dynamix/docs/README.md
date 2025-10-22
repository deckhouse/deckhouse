---
title: "Cloud provider — Dynamix"
description: "Management of virtual servers and containers in the Deckhouse Kubernetes Platform using Dynamix."
---

The `cloud-provider-dynamix` module is responsible for interacting with the Dynamix resources. It allows the [node manager](../../modules/node-manager/) module to use Dynamix resources for provisioning nodes for the specified [node group](../../modules/node-manager/cr.html#nodegroup) (a group of nodes that are acted upon as if they were a single entity).
