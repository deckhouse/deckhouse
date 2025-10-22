---
title: "Cloud provider — zVirt"
description: "Managing cloud resources in Deckhouse Kubernetes Platform based on zVirt."
---

The `cloud-provider-zvirt` module is responsible for interacting with the zVirt resources. It allows the [node manager](../../modules/node-manager/) module to use zVirt resources for provisioning nodes for the specified [node group](../../modules/node-manager/cr.html#nodegroup) (a group of nodes that are acted upon as if they were a single entity).
