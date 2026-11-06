---
title: Cloud cluster
description: |
  A cluster with the [clusterType: Cloud](reference/api/cr.html#clusterconfiguration-clustertype). Deckhouse Platform has access to the API of the cloud infrastructure and creates and manages the virtual machines of the nodes.
lang: en
---

The [installer](glossary.html#installer) creates all the required resources, such as virtual machines and networks, from the `<PROVIDER>ClusterConfiguration` of the [cloud provider](glossary.html#cloud-provider). The master nodes of a cloud cluster are [CloudPermanent nodes](glossary.html#cloud-permanent-node), worker nodes are usually [CloudEphemeral nodes](glossary.html#cloud-ephemeral-node).

For details, see [Platform installation](installing/).
