---
title: "Overview"
permalink: en/virtualization-platform/documentation/admin/platform-management/platform-scaling/overview.html
---

Deckhouse Virtualization Platform (DVP) provides built-in mechanisms for comprehensive management of cluster architecture — both at the control plane level and at the node level.

Management capabilities include:

- [Control plane management](./control-plane/control-plane-management-and-configuration.html) — automation of configuration and updates for Kubernetes API, etcd, and other system components; issuance and renewal of certificates; scaling of the control plane and transitioning between single-master and multi-master modes; high availability and recovery.
- [Node management](./node/node-management.html) — creation and scaling of node groups using NodeGroup objects; automatic deployment and updating of nodes; management of static nodes; use of additional tools for fine-tuning.

These capabilities allow the creation of reliable, scalable, and self-healing clusters, adapting them to any requirements for performance, high availability, and infrastructure constraints.

The following sections provide detailed descriptions of features, configuration examples, and best practices for effective control plane and node management in Deckhouse Virtualization Platform.
