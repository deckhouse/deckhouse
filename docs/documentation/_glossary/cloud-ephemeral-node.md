---
title: CloudEphemeral node
description: A temporary cloud node that Deckhouse Platform orders, creates and deletes automatically through the cloud provider API. It is described by a [NodeGroup](/modules/node-manager/cr.html#nodegroup) with the CloudEphemeral node type.
lang: en
---

The parameters of the virtual machines are taken from an [instance class](glossary.html#instance-class), the number of nodes and their zones from the `cloudInstances` section of the [node group](glossary.html#node-group). The nodes are created by Machine Controller Manager or Cluster API, which are part of the [`node-manager`](/modules/node-manager/) module.

CloudEphemeral groups support [node group autoscaling](glossary.html#node-group-autoscaling), [standby nodes](glossary.html#standby-node) and [chaos engineering](glossary.html#chaos-engineering).

For details, see [Adding and managing cloud nodes](admin/configuration/platform-scaling/node/cloud-node.html).
