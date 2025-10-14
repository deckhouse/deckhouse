---
title: Overview
permalink: en/admin/configuration/high-reliability-and-availability/
description: High reliability and availability
---

The reliability and resilience of a Kubernetes cluster are key characteristics
that define the stability of the infrastructure.
Deckhouse Kubernetes Platform (DKP) ensures high availability (HA) and fault tolerance
through built-in mechanisms and modules.

When HA mode is enabled, critical cluster components are launched with the necessary redundancy
to guarantee their continuous operation.
Even if a single instance fails, services can continue functioning without downtime.
For more information on enabling HA mode, refer to [Managing HA mode](enable.html).

If the cluster has more than one master node, HA mode is automatically enabled,
both during the initial deployment and when additional nodes are added later.
Recommended roles and number of nodes can be found in [Recommendations for configuring cluster nodes and preventing overload](recommendations.html).

DKP provides chaos engineering tools to test cluster resilience.
These tools let you deliberately or randomly disrupt individual components and observe the infrastructure's response.
For information on configuring these tools, refer to [Chaos engineering](chaos-engineering.html).

You can further increase cluster fault tolerance by enabling inter-cluster communication
based on the Service Mesh mode of the [`istio`](/modules/istio/) module.
In this mode, federation is configured between multiple clusters.
In case of failures in one cluster, the load is automatically redistributed to others.
For configuration details, refer to [Federation](../network/alliance/federation.html).
