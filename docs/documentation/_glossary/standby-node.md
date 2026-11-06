---
title: Standby node
description: A pre-provisioned node kept in the cluster so that new application replicas start without waiting for node creation during horizontal scaling. The number of standby nodes is set by [cloudInstances.standby](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances).
lang: en
---

Standby nodes are available for [CloudEphemeral nodes](glossary.html#cloud-ephemeral-node). The number is set as an absolute value or as a percentage of the maximum number of nodes in the [node group](glossary.html#node-group). At least one node of the group must already be running. The share of node resources taken by service components outside Deckhouse Platform is set by `standbyHolder.overprovisioningRate`. Standby nodes increase infrastructure costs.

For details, see [Adding and managing cloud nodes](admin/configuration/platform-scaling/node/cloud-node.html).
