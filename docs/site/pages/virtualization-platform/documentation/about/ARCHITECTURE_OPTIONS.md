---
title: "Architecture options"
permalink: en/virtualization-platform/documentation/about/architecture-options.html
---

Deckhouse Virtualization Platform (DVP) supports several cluster architecture options — from single-node installations to distributed configurations. The choice of architecture depends on the requirements: quick test deployments, high availability, or isolation of system services from user workloads.

## Core components

The minimal installation includes all the essential components required to run and operate DVP:

- Control plane and auxiliary components.
- SDS components and external storage systems.
- Networking modules.
- Virtualization module.
- Web interface.
- Monitoring and logging tools.
- Certificate management.

## Minimum requirements

Before choosing an architecture, review the [minimum platform requirements](/products/virtualization-platform/documentation/about/requirements.html#minimum-platform-requirements). These requirements define the minimal resources necessary to launch the platform. They may be adjusted to account for potential peak loads, growth in the number of VM users, and infrastructure components.

## Single-node (Edge) cluster

In a single-node configuration, all management components, auxiliary services, and virtual machines run on a single server. This architecture is suitable for test environments as well as edge scenarios. It requires minimal resources and allows rapid cluster deployment.

**Advantages:**
- All components can run on a single node.
- Simple installation.
- Minimal infrastructure costs.

**Disadvantages:**
- No high availability.
- In case of platform oversubscription, user workloads may be affected.

## Cluster with one master node and worker nodes

In this architecture, one node handles management functions, while virtual machines are deployed on dedicated worker nodes. This option is suitable for small clusters where system services and user workloads need to be separated.

**Advantages:**
- Clear separation of system services and workloads.

**Disadvantages:**
- No high availability.
- Additional resources required for management and auxiliary components.

## Three-node cluster (High Availability)

An architecture with three master nodes is used when high availability is required. Management components are distributed across three servers, ensuring fault tolerance of the cluster control plane and continued operation if one node fails. User workloads can run on the same servers or on dedicated worker nodes.

**Advantages:**
- High availability.

**Disadvantages:**
- In case of platform oversubscription, user workloads may still be affected.

## Highly available distributed cluster

The highly available distributed architecture is applied in large clusters. Management components are deployed on three dedicated master nodes. If necessary, system services, monitoring, and ingress can be moved to separate system or frontend nodes. User virtual machines are executed exclusively on worker hypervisors.

**Advantages:**
- High availability and scalability.
- Isolation of user workloads from system services.
- Failure domain separation.

**Disadvantages:**
- Higher resource and infrastructure requirements.
