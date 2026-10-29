---
title: Active-standby balancing
description: A scheme of balancing incoming traffic in a multicluster in which the external load balancer routes all traffic to the currently active cluster, and the standby cluster receives traffic only when the active one becomes unavailable.
lang: en
---

In the active-standby scheme the active cluster of the [multicluster](glossary.html#multicluster) receives all traffic, and the standby cluster stays idle until the active one fails. Traffic failover to the standby cluster happens automatically when problems are detected. The balancing is configured by the cluster administrator as part of [geo-reserving](glossary.html#geo-reserving). The alternative scheme is [active-active](glossary.html#active-active).

For details, see [Geo-reserving](architecture/disaster-resilience/geo-reserving.html).
