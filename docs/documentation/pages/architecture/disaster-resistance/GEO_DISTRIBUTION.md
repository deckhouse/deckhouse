---
title: Geo-distribution
permalink: en/architecture/disaster-resilience/geo-distribution.html
---

Geo-distribution, in the context of disaster resilience in Deckhouse Kubernetes Platform (DKP),
is an architectural approach that ensures continuous cluster operation by distributing its components
across multiple [availability zones (Multi-AZ)](#using-multiple-availability-zones-multi-az) or [regions (Multi-Region)](#using-multiple-regions-multi-region).
This approach minimizes the risks of incidents of various scales.

{% alert level="warning" %}
When implementing geo-distribution, it’s important to ensure network connectivity between cluster nodes.
This is the responsibility of the platform administrator.
{% endalert %}

## Using multiple availability zones (Multi-AZ)

DKP lets you distribute cluster nodes across availability zones.
This approach increases application resilience to infrastructure-wide failures
and ensures the applications remain functional even if one of the zones in a data center or cloud becomes unavailable.

An **availability zone (AZ)** is the basic unit of a disaster-resilient architecture.
It represents a logical group of one or several data centers located within a few kilometers of each other.
An AZ can be viewed as an isolated unit (a virtual data center), even though it’s a physically distributed infrastructure.
Availability zones are part of [regions](#using-multiple-regions-multi-region).

### Availability zone features

Availability zones are defined by the following features:

- Sufficient physical separation between data centers within the same AZ (usually ≤10 km)
  to protect from localized incidents (such as floods or fires), while maintaining low network latency.
- Independent power supply for components within the zone (different substations, diesel generators, etc.).
- Use of different providers and network routes by components within the zone.
- Relatively low latency within the zone.

### Distributing nodes across zones

With most cloud providers, zone distribution happens automatically.
In non-cloud environments, administrators must manually label nodes to indicate the target zone.

For the node labeling process and how to add nodes to groups, refer to ["Node management"](../../admin/configuration/platform-scaling/node/node-management.html).

## Using multiple regions (Multi-Region)

DKP supports distributing cluster nodes across different regions.
This approach allows applications to continue running
even in the event of a complete outage of a data center or cloud provider.

A **region** is an independent geographic area
that includes several (typically at least three) isolated [availability zones (AZs)](#using-multiple-availability-zones-multi-az).
All zones within a region are united into a single logical infrastructure unit with shared management services.
A region is a key element in the global distributed architecture used by cloud providers.

{% alert level="info" %}
Using multiple regions (Multi-Region) is usually appropriate
only when applications can work independently in each region (without reaching out to others).
This is due to the relatively high latency between regions.
Typical use cases include hosting CDN cache servers or distributed monitoring nodes.
{% endalert %}

### Region features

Regions are defined by the following features:

- Latency differences within a region and between regions.
  Within a region, latency between AZs can be around 5 ms.
  Between regions, it can be between 50 and 200 ms.
- Geographic distance. Regions may be widely dispersed.
  For example, the distance between `europe-west3` (Frankfurt) and `us-east-1` (Virginia) is around 6500 km.
- Physical safety. Providers try to place regions outside of seismically or politically unstable areas.
- Energy redundancy. Regions are usually connected to multiple national power grids
  (for example, `me-central-1` (UAE) uses 4 independent power sources).

### Distributing nodes across regions

Distributing nodes across regions requires the following:

- Labeling each node to indicate the region it belongs to.
- Providing stable communication channels between regions.

For the node labeling process and how to add nodes to groups, refer to ["Node management"](../../admin/configuration/platform-scaling/node/node-management.html).

## Balancing incoming traffic

In a geo-distributed cluster, traffic balancing can be handled either by the cloud provider
or by internal load balancers (such as MetalLB).
The specific implementation depends on the infrastructure and must be configured by the administrator.
For details on traffic balancing options and configuration, refer to [Network](../../admin/configuration/network/).

## Storage organization

Storage system organization and configuration in a geo-distributed cluster are handled by the administrator.
DKP supports various storage options and the choice depends on the specific project needs.
For details on supported storage systems, their features and configuration, refer to [Storage](../../admin/configuration/storage/).
