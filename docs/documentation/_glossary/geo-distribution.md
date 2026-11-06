---
title: Geo-distribution
description: An approach to disaster resilience in which the components of a cluster are distributed across several availability zones (Multi-AZ) or regions (Multi-Region) so that the cluster keeps running when a zone or region fails.
lang: en
---

Geo-distribution reduces the impact of infrastructure failures on application availability. Distributing nodes across [availability zones](glossary.html#availability-zone) protects applications from the failure of one zone in a data center or cloud. Distributing them across [regions](glossary.html#region) lets applications survive the outage of a whole data center or cloud provider, provided that the application can work independently in each region.

The platform administrator is responsible for the network connectivity between cluster nodes, for the balancing of incoming traffic by the [cloud provider](glossary.html#cloud-provider) or an internal load balancer, and for the organization of storage. Geo-distribution is one of the two approaches to [disaster resilience](glossary.html#disaster-resilience) in Deckhouse Platform. The other is [geo-reserving](glossary.html#geo-reserving).

For details, see [Geo-distribution](architecture/disaster-resilience/geo-distribution.html).
