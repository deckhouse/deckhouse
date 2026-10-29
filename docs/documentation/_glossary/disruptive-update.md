---
title: Disruptive update
description: A node update that requires node downtime and may briefly interrupt system components, for example upgrading containerd or kubelet across several versions. The approval mode is set by the [disruptions.approvalMode](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions-approvalmode) parameter of the [NodeGroup](/modules/node-manager/cr.html#nodegroup).
lang: en
---

Regular node updates are always applied automatically and cause no downtime or reboot. The `disruptions` section governs only disruptive updates, such as a kernel or containerd update or a significant kubelet version change. Only one node per group is updated at a time.

Three approval modes are available in a [node group](glossary.html#node-group). In `Manual` mode the `NodeRequiresDisruptionApprovalForUpdate` [alert](glossary.html#alert) is raised, and the update starts after the `update.node.deckhouse.io/disruption-approved=` annotation is added to the node. The node is not drained automatically. In `Automatic` mode the node is drained and updated without confirmation. In `RollingUpdate` mode a new node with the updated settings is created and the old one is removed. This mode applies to cloud nodes only.

Use `Manual` mode for the groups of [master nodes](glossary.html#master-node).

For details, see [Node management basics](admin/configuration/platform-scaling/node/node-management.html).
