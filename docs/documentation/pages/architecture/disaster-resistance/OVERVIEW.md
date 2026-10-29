---
title: Disaster resilience
permalink: en/architecture/disaster-resilience/
---

**[Disaster resilience](../../glossary.html#disaster-resilience)** is the ability of infrastructure based on Deckhouse Platform (DP)
to remain operational in the event of large-scale failures.
It is achieved through distributed deployment, automatic traffic switching, and replication of critical components.

DP implements two approaches to disaster resilience:

- [Geo-distribution](../../glossary.html#geo-distribution), which is a distribution of infrastructure components
  across different [availability zones](../../glossary.html#availability-zone) (Multi-AZ) or [regions](../../glossary.html#region) (Multi-Region).
  This approach helps reduce the impact of infrastructure failures on application availability.
  For details, refer to [Geo-distribution](../../architecture/disaster-resilience/geo-distribution.html).

- [Geo-reserving](../../glossary.html#geo-reserving), which is based on using multiple independent clusters combined into a [multi-cluster](../../glossary.html#multicluster) system.
  If one of the clusters becomes unavailable, traffic can be automatically redirected to another.
  For details, refer to [Geo-reserving](../../architecture/disaster-resilience/geo-reserving.html).

Both approaches require configuring network connectivity between nodes and regions,
traffic balancing, and proper storage organization.
The choice of an architecture depends on the specifics of applications
and infrastructure constraints.
