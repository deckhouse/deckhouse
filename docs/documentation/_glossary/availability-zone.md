---
title: Availability zone
description: "The basic unit of a disaster-resilient architecture: a logical group of one or several data centers located within a few kilometers of each other and viewed as an isolated unit. Availability zones are part of regions."
lang: en
---

An availability zone (AZ) is physically distributed infrastructure that can be treated as a virtual data center. The data centers of one zone are separated far enough (usually 10 km or less) to survive local incidents such as floods or fires, keep low network latency between each other, have independent power supply and use different network providers and routes. Several zones form a [region](glossary.html#region).

Deckhouse Platform lets you distribute cluster nodes across availability zones (Multi-AZ) so that applications keep running when one zone of a data center or cloud becomes unavailable. With most [cloud providers](glossary.html#cloud-provider) the zone distribution happens automatically. Outside clouds the administrator labels nodes with their zone. This is one form of [geo-distribution](glossary.html#geo-distribution).

For details, see [Geo-distribution](architecture/disaster-resilience/geo-distribution.html).
