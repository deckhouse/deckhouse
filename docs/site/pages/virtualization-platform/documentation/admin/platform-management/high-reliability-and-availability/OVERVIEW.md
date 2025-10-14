---
title: Overview
permalink: en/virtualization-platform/documentation/admin/platform-management/high-reliability-and-availability/
description: High reliability and availability
lang: en
---

The reliability and resilience of a Kubernetes cluster are key characteristics
that define the stability of the infrastructure.
Deckhouse Virtualization Platform (DVP) ensures high availability (HA) and fault tolerance
through built-in mechanisms and modules.

When HA mode is enabled, critical cluster components are launched with the necessary redundancy
to guarantee their continuous operation.
Even if a single instance fails, services can continue functioning without downtime.
For more information on enabling HA mode, refer to [Managing HA mode](enable.html).

If the cluster has more than one master node, HA mode is automatically enabled,
both during the initial deployment and when additional nodes are added later.
Recommended roles and number of nodes can be found in [Recommendations for configuring cluster nodes and preventing overload](recommendations.html).
