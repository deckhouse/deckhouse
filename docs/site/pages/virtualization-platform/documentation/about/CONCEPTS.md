---
title: "Concepts"
permalink: en/virtualization-platform/documentation/about/concepts.html
---

Deckhouse Virtualization Platform (DVP) supports several cluster architecture options — from single-node deployments to distributed configurations. The specific option is chosen depending on the requirements: the need for quick test deployment, ensuring high availability, or isolating system services from user workloads.

## Core components

A minimal installation includes all the core components required to run and operate DVP:

- Control plane and auxiliary components.
- SDS components and external storage systems.
- Networking modules.
- Virtualization module.
- Web interface.
- Monitoring and logging tools.
- Certificate management.

## Minimum requirements

Before choosing an architecture, review the [minimum platform requirements](/products/virtualization-platform/documentation/about/requirements.html#minimum-platform-requirements).

## Single-node (Edge) cluster

In a single-node configuration, all control plane components, auxiliary services, and virtual machines are hosted on a single server. This architecture is typically used for testing environments as well as edge scenarios. It requires minimal resources and enables quick cluster deployment.

**Advantages:**
- Ability to host all components on a single node.
- Simple installation.
- Minimal infrastructure costs.

**Disadvantages:**
- No high availability.
- Platform resource overcommitment may impact user workloads.

## Cluster with one master node and worker nodes

In this architecture, one node serves as the control plane, while virtual machines are deployed on dedicated worker nodes. This option is suitable for small clusters where separation of system services and user workloads is required.

**Advantages:**
- Clear separation of system services and workloads.

**Disadvantages:**
- No high availability.
- Additional resources required for control plane and auxiliary components.

## Three-master cluster (High Availability)

A three-master-node architecture is used when high availability is required. Control plane components are distributed across three servers, ensuring fault tolerance of the control plane and continued operation if one node fails.  
User workloads can run either on the same servers or on dedicated worker nodes.

**Advantages:**
- High availability of the control plane.

**Disadvantages:**
- Platform resource overcommitment may affect user workloads.

## Distributed Cluster

The distributed architecture is designed for large-scale clusters. Control plane components are deployed on three dedicated master nodes, while system services, monitoring, and ingress can be moved to separate system or frontend nodes.  
User virtual machines are executed exclusively on worker hypervisors.

**Advantages:**
- High availability and scalability.
- Isolation of user workloads from system services.
- Separation of failure domains.

**Disadvantages:**
- Increased resource and infrastructure requirements.  
