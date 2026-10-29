---
title: Software-defined network (SDN)
description: An additional network segment for pods and virtual machines that the administrator describes declaratively in custom resources, while the [`sdn`](/modules/sdn/) module configures the nodes.
lang: en
---

Software-defined networks in Deckhouse Platform cover the configuration of network interfaces on nodes (port aggregation, bridges, VLAN interfaces), additional networks for workloads that are available in the whole cluster or within one namespace (project networks), IP address management for these networks, underlay networks for passing physical interfaces to applications, and system networks for service traffic at the node level.

For details, see [Software-defined networking](admin/configuration/network/sdn/).
