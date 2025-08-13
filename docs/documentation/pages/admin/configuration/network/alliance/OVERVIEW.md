---
title: "Overview"
permalink: en/admin/configuration/network/alliance/
---

Deckhouse Kubernetes Platform (DKP) supports two inter-cluster communication models:

- [Multicluster](../alliance/multicluster.html)
- [Federation](../alliance/federation.html)

Both models can be implemented using Istio (via the [`istio`](/modules/istio/) module) or Cilium (via the [`cni-cilium`](/modules/cni-cilium/) module).
Both tools provide deployment of a service mesh for managing and configuring network interactions between applications within a cluster.

## Service mesh usage specifics in DKP

Available scenarios depend on the platform edition.

Multicluster and federation are available in the Enterprise Edition (EE).

In the Community Edition (CE), Basic Edition (BE), Standard Edition (SE), and Standard Edition+ (SE+),
the service mesh can only be used within a single cluster — for example, to implement patterns such as Circuit Breaker,
Canary Deployment, and others.

| Use case | CE / BE / SE / SE+  | EE |
|----------|---------------------|----|
| Service mesh in one cluster | +  | +  |
| Federation                  | -  | +  |
| Multicluster                | -  | +  |

## Differences between federation and multicluster

Federation connects independent (sovereign) clusters:

- Each cluster has its own namespace and services.
- Inter-cluster communication is configured explicitly.
  You define which services are accessible externally.

Multicluster tightly connects integrated clusters:

- A shared namespace is used.
- Services from one cluster are available to others as if they were local (unless restricted by authorization policies).

## Models for combining multiple clusters (Istio example)

When using Istio, different combination models are possible:

- Network models: Clusters can be in the same or different networks.
- Control plane models: One or multiple service mesh control planes.
- Isolation models: Tenant isolation at the namespace, network, or cluster level.
- Service mesh models: One shared service mesh or multiple meshes.

These parameters can be combined to build an architecture tailored to the requirements of a specific infrastructure.

## Problems solved with multicluster and federation

Combining clusters into a federation or multicluster can help solve the following tasks:

- Geographical distribution and GEO load balancing: Clusters serve users closer to their location.
- Fault tolerance: If a cluster or data center fails, traffic automatically switches to others (automatic failover).
- Horizontal scaling: Load distribution across clusters increases overall system performance.
- Environment and resource isolation: Different environments (production, staging, dev) can be placed in separate clusters,
  with optional interconnection.
- Regulatory compliance: For example, data storage requirements for specific geographic zones.
- Hybrid and multi-cloud infrastructure creation: Combining clusters located in different environments
  (cloud, different providers, etc.).
