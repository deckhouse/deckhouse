---
title: Fencing
description: A mechanism that isolates a node that has lost connectivity with the cluster. The node is shut down and, if needed, its Node object is deleted, so that no workload keeps running on it in an undefined state.
lang: en
---

Fencing works in two stages: the node is first marked as suspicious at the gossip protocol level, then isolated. It is enabled per [node group](glossary.html#node-group) with the `spec.fencing.mode` parameter of the [NodeGroup](/modules/node-manager/cr.html#nodegroup). Fencing is required for [ColdStandby](glossary.html#coldstandby): without it an unavailable virtual machine stays on the failed node and resumes together with it.

For details, see [Virtual machine reliability](admin/configuration/high-reliability-and-availability/vm-reliability.html).
