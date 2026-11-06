---
title: Static node
description: A node on a bare-metal server or a virtual machine that is created outside Deckhouse Platform automation and connected to the cluster manually or through CAPS. It is described by a [NodeGroup](/modules/node-manager/cr.html#nodegroup) with the Static node type.
lang: en
---

Static nodes do not support automatic scaling and updates: kubelet updates, restarts and replacements are performed manually or by external tools. A node is connected either by running the [bootstrap script](glossary.html#bootstrap-script) on the server or automatically by [CAPS](glossary.html#caps). In a cloud environment, cloud-controller-manager does not manage a Static node even if a [cloud provider](glossary.html#cloud-provider) module is enabled.

A [node group](glossary.html#node-group) can combine static nodes with [CloudStatic nodes](glossary.html#cloud-static-node), which is the basis of a [hybrid cluster](glossary.html#hybrid-cluster).

For details, see [Adding and managing bare-metal nodes](admin/configuration/platform-scaling/node/bare-metal-node.html).
