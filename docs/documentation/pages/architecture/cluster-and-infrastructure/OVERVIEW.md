---
title: Cluster & Infrastructure subsystem
permalink: en/architecture/cluster-and-infrastructure/
search: cluster & infrastructure, node management
description: Architecture of the Cluster & Infrastructure subsystem in Deckhouse Kubernetes Platform.
---

This section describes the architecture of the Cluster & Infrastructure subsystem of the Deckhouse Kubernetes Platform (DKP).

The Cluster & Infrastructure subsystem is responsible for the infrastructure layer of Kubernetes cluster management. Cluster node management is implemented using the [`node-manager`](/modules/node-manager/) module, while interaction with IaaS providers is handled by the corresponding `cloud-provider-` family modules.

This section describes the mechanisms for managing all node types used in DKP, as well as [hybrid node groups and clusters](hybrid-nodegroups-and-clusters/).

The Cluster & Infrastructure subsystem also includes the following modules:

* [`chrony`](/modules/chrony/): Provides time synchronization across all cluster nodes.
* [`registry-packages-proxy`](/modules/registry-packages-proxy/): Provides an internal proxy server for container registry packages.
* [`terraform-manager`](/modules/terraform-manager/): Provides tools for managing Terraform state within a Kubernetes cluster.

This subsection also describes the [Bashible](bashible/) service, which is a key component of the Cluster & Infrastructure subsystem. Bashible is used by the [`node-manager`](/modules/node-manager/) module to manage node configuration.
