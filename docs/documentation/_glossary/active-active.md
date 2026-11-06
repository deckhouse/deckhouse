---
title: Active-active balancing
description: A scheme of balancing incoming traffic in a multicluster in which the external load balancer distributes traffic across several active clusters at once, for example by geographic proximity or current load.
lang: en
---

In the active-active scheme all clusters of the [multicluster](glossary.html#multicluster) process traffic simultaneously. Traffic goes to the nearest or least loaded cluster, or follows other defined rules. Shared data is replicated between the clusters. When one cluster fails, traffic is automatically rerouted to the others. The balancing is configured by the cluster administrator as part of [geo-reserving](glossary.html#geo-reserving). The alternative scheme is [active-standby](glossary.html#active-standby).

For details, see [Geo-reserving](architecture/disaster-resilience/geo-reserving.html).
