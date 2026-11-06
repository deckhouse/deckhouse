---
title: CloudPermanent node
description: A permanent cloud node whose configuration is taken from the provider cluster configuration rather than from a [NodeGroup](/modules/node-manager/cr.html#nodegroup). The master nodes of a cloud cluster are CloudPermanent nodes.
lang: en
---

CloudPermanent nodes are described in the `nodeGroups` section of the `<PROVIDER>ClusterConfiguration` structure, and their number is set by `replicas`. The nodes are managed by Terraform built into Deckhouse Platform, and configuration changes are applied by running the [installer](glossary.html#installer) with `dhctl converge`.

For details, see [Adding and managing cloud nodes](admin/configuration/platform-scaling/node/cloud-node.html).
