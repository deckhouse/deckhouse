---
title: Geo-reserving
permalink: en/architecture/disaster-resilience/geo-reserving.html
---

**Geo-reserving**, in the context of disaster resilience in Deckhouse Kubernetes Platform (DKP),
is a method of increasing fault tolerance by combining multiple independent Kubernetes clusters
into a single multi-cluster system (multi-cluster) distributed across different geographic locations.

DKP offers several tools to unite multiple clusters into a multi-cluster.
This approach ensures automatic routing of both external and internal traffic to another cluster
with an available copy of the application, in case it becomes unavailable in the original cluster.

Clusters can be combined using:

- The built-in Istio-based Service Mesh.
- DKP’s networking features powered by Cilium.

Enabling multi-cluster functionality requires deploying one DKP cluster per region
and then connecting them using a declarative API.
It is important that the application can run in parallel across different regions,
and that there is a stable network connection between the regions.
A properly configured system ensures global disaster resilience for the application,
guaranteeing uninterrupted operation even if an entire region or cloud becomes unavailable.

For the process of joining multiple clusters into a multi-cluster, refer to [Inter-cluster cooperation](../../admin/configuration/network/alliance/).

## Balancing incoming traffic in a multi-cluster

To ensure disaster resilience in a multi-cluster, incoming traffic can be balanced by an external load balancer
using one of the following methods: `active-active` or `active-standby`.
The balancing configuration is handled by the cluster administrator.

### Balancing traffic via active-active method

Following the `active-active` method, the external load balancer distributes traffic across multiple active clusters,
for example, based on geographic proximity or current workload.

![Active-active balancing](../../images/architecture/active-active-balancing.png)

Key aspects of `active-active` balancing:

- All clusters in the multi-cluster process traffic simultaneously.
- Traffic is routed to the nearest and least loaded cluster, or according to other defined rules.
- Shared data is replicated across clusters.
- In the event of a failure in one cluster, traffic is automatically rerouted to the others (automatic failover).

### Balancing traffic via active-standby method

Following the `active-standby` method, the external load balancer routes traffic only to the currently active cluster.
The standby cluster remains idle and receives traffic only if the primary cluster becomes unavailable.

![Active-standby balancing](../../images/architecture/active-standby-balancing.png)

Key aspects of `active-standby` balancing:

- The active cluster receives all traffic.
- The standby cluster remains idle until the primary cluster fails.
- Traffic failover to the standby cluster is performed automatically upon detection of issues.

## Storage organization in a multi-cluster

Storage system organization and configuration in a multi-cluster are handled by the administrator.
DKP supports various storage systems.
The choice of a suitable solution depends on requirements of performance,
fault tolerance, and data synchronization methods.
For details on supported storage systems, their features and configuration, refer to [Storage](../../admin/configuration/storage/).
