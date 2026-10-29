---
title: Node group autoscaling
description: Automatic change of the number of nodes in a group by the Cluster Autoscaler component of the [`node-manager`](/modules/node-manager/) module, based on CPU and memory demand. It triggers only when Pending pods cannot be scheduled on the existing nodes.
lang: en
---

Autoscaling is available for [CloudEphemeral nodes](glossary.html#cloud-ephemeral-node). The limits are set in the `cloudInstances` section of the [node group](glossary.html#node-group): `minPerZone` and `maxPerZone` bound the number of nodes per zone, `maxUnavailablePerZone` limits unavailable nodes during changes, `priority` defines which groups Cluster Autoscaler scales first. Autoscaling does not apply to [static nodes](glossary.html#static-node): for them [CAPS](glossary.html#caps) maintains the number set by `staticInstances.count`.

For details, see [Adding and managing cloud nodes](admin/configuration/platform-scaling/node/cloud-node.html).
