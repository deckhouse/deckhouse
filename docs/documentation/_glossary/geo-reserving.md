---
title: Geo-reserving
description: A method of increasing fault tolerance in which several independent Kubernetes clusters in different geographic locations are combined into a multicluster, and traffic is automatically redirected to a cluster that has an available copy of the application.
lang: en
---

Geo-reserving requires one Deckhouse Platform cluster per region, connected with a declarative API into a [multicluster](glossary.html#multicluster): either through the built-in [service mesh](glossary.html#service-mesh) based on Istio or through the networking features of the platform based on Cilium. The application must be able to run in parallel in different regions, and a stable network channel must exist between the regions. A properly configured system keeps the application running even when a whole [region](glossary.html#region) or cloud becomes unavailable.

Incoming traffic in a multicluster is balanced by an external load balancer in one of two schemes, [active-active](glossary.html#active-active) or [active-standby](glossary.html#active-standby). The balancing and the storage organization are configured by the cluster administrator. Geo-reserving is one of the two approaches to [disaster resilience](glossary.html#disaster-resilience). The other is [geo-distribution](glossary.html#geo-distribution).

For details, see [Geo-reserving](architecture/disaster-resilience/geo-reserving.html).
