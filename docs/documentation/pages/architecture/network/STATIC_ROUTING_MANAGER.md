---
title: Static-routing-manager module
permalink: en/architecture/network/static-routing-manager.html
search: static-routing-manager, static routes, ip rule
description: Architecture of the static-routing-manager module in Deckhouse Platform.
---

The `static-routing-manager` module manages static routes and `ip rule` entries on cluster nodes.

The module works with the following custom resources of the `network.deckhouse.io` API group:

- [IPRuleSet](/modules/static-routing-manager/cr.html#ipruleset): Describes a set of `ip rule` entries to be created on nodes matching a `nodeSelector`.
- [RoutingTable](/modules/static-routing-manager/cr.html#routingtable): Describes a routing table and the routes it contains, for nodes matching a `nodeSelector`.

The module also works with the following internal custom resources of the `internal.network.deckhouse.io` API group:

- SDNInternalNodeIPRuleSet: Contains the set of `ip rule` entries for a specific cluster node.
- SDNInternalNodeRoutingTable: Contains the routing table for a specific cluster node.

For more details about module configuration and usage examples, refer to the [corresponding documentation section](/modules/static-routing-manager/).

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`static-routing-manager`](/modules/static-routing-manager/) module and its interactions with other components of Deckhouse Platform are shown in the following diagram:

![Static-routing-manager module architecture](../../images/architecture/network/c4-l2-static-routing-manager.svg)

## Module components

The module consists of one **agent** (DaemonSet) component and one **agent** container.
Agent runs on every cluster node and performs the following actions:

- Watches SDNInternalNodeRoutingTable and SDNInternalNodeIPRuleSet custom resources and updates their status.
- Applies the static routes and `ip rule` entries to the node's kernel routing table.

The SDNInternalNodeRoutingTable and SDNInternalNodeIPRuleSet custom resources are created by the Deckhouse controller of the [`deckhouse`](/modules/deckhouse/) module for each cluster node matching the `nodeSelector` of the IPRuleSet and RoutingTable resources.

## Module interactions

The module interacts with **kube-apiserver**:

- Manages the SDNInternalNodeRoutingTable and SDNInternalNodeIPRuleSet custom resources.
- Watches the IPRuleSet and RoutingTable custom resources and updates their status.
