---
title: Region
description: An independent geographic area that includes several, typically at least three, isolated availability zones united into a single logical infrastructure unit with shared management services.
lang: en
---

A region is a key element of the global distributed architecture of [cloud providers](glossary.html#cloud-provider). Regions are widely dispersed geographically, placed outside seismically or politically unstable areas and connected to several power grids. Latency between the [availability zones](glossary.html#availability-zone) of one region is about 5 ms, between regions it is 50 to 200 ms.

Deckhouse Platform supports distributing cluster nodes across regions (Multi-Region) so that applications keep running even when a whole data center or cloud provider fails. Because of the latency between regions, this is appropriate only for applications that work independently in each region, for example CDN cache servers or distributed [monitoring nodes](glossary.html#monitoring-node). The administrator labels each node with its region and provides stable communication channels between regions.

For details, see [Geo-distribution](architecture/disaster-resilience/geo-distribution.html).
