---
title: Node-manager module
permalink: en/architecture/cluster-and-infrastructure/node-management/node-manager.html
search: node-manager architecture
description: Architecture of the node-manager module in Deckhouse Kubernetes Platform.
---

Cluster nodes are managed by the `node-manager` module.

For a detailed description of the module's functions, configuration options, and usage examples, refer to the [corresponding documentation section](/modules/node-manager/).

## Module architecture

The module architecture varies depending on the node type and differs in terms of its component composition. The following pages describe the module architecture for different node types:

* [Managing CloudEphemeral nodes](cloud-ephemeral-nodes.html)
* [Managing CloudPermanent nodes](cloud-permanent-nodes.html)
* [Managing CloudStatic nodes](cloud-static-nodes.html)
* [Managing Static nodes](static-nodes.html)
* [Managing hybrid node groups and clusters](hybrid-nodegroups-and-clusters.html)
