---
title: Disaster resilience
description: The ability of infrastructure based on Deckhouse Platform to remain operational in the event of large-scale failures. It is achieved through distributed deployment, automatic traffic switching and replication of critical components.
lang: en
---

Deckhouse Platform implements two approaches to disaster resilience. [Geo-distribution](glossary.html#geo-distribution) spreads the components of one cluster across several [availability zones](glossary.html#availability-zone) or [regions](glossary.html#region). [Geo-reserving](glossary.html#geo-reserving) combines several independent clusters into a [multicluster](glossary.html#multicluster) and redirects traffic to another cluster when one becomes unavailable.

Both approaches require network connectivity between nodes and regions, traffic balancing and a suitable storage organization. The choice of an architecture depends on the applications and the constraints of the infrastructure. Within one cluster, the redundancy of critical components is provided by the [HA mode](glossary.html#ha-mode).

For details, see [Disaster resilience](architecture/disaster-resilience/).
