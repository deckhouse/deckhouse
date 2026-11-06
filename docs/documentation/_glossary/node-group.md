---
title: Node group
description: A set of cluster nodes with common parameters, described by a [NodeGroup](/modules/node-manager/cr.html#nodegroup). The NodeGroup defines the node type, the number of nodes and their configuration, and Deckhouse Platform keeps the nodes in line with it.
lang: en
---

Node groups divide the cluster infrastructure logically. Typical groups are `master` for the [control plane](glossary.html#control-plane), `front` for [frontend nodes](glossary.html#frontend-node), `monitoring` for [monitoring nodes](glossary.html#monitoring-node), `system` for [system nodes](glossary.html#system-node) and `worker` for user applications.

Each group has centralized settings: the Kubernetes version, resources, taints, labels and kubelet parameters. When a NodeGroup is created or changed, the [`node-manager`](/modules/node-manager/) module brings the nodes of the group to the specified configuration. The node type is one of [Static](glossary.html#static-node), [CloudEphemeral](glossary.html#cloud-ephemeral-node), [CloudPermanent](glossary.html#cloud-permanent-node) or [CloudStatic](glossary.html#cloud-static-node).

For details, see [Node management basics](admin/configuration/platform-scaling/node/node-management.html).
