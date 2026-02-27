---
title: "Software-defined networking (SDN)"
permalink: en/admin/configuration/network/sdn/
description: |
  Software-defined networking (SDN) in Deckhouse Kubernetes Platform: overview of capabilities and implementation methods.
search: additional networks, DPDK
---

The Deckhouse Kubernetes Platform supports adding additional software-defined networks (SDN) to the cluster. The functions of additional software-defined networking (hereinafter referred to as additional networks) within DKP are implemented using the [`sdn`](/modules/sdn/) module.

Software-defined networking in DKP allow declarative management of additional network segments for application workloads (pods, virtual machines). Instead of manually configuring network interfaces on each cluster node, the administrator describes the desired network state through custom Kubernetes resources, and the [`sdn`](/modules/sdn/) module automatically configures the necessary network equipment.

DKP supports the following features for working with software-defined networks:

* Configuration of network interfaces on nodes. Features such as port aggregation, bridging network interfaces, and configuring VLAN interfaces are supported.
* Additional networks for application workloads. Supports adding additional software-defined networks to the cluster: publicly available in each project (cluster) and available within a single namespace (project network).
* Underlay networks for hardware device passthrough. This allows DPDK applications and other high-performance workloads to directly access physical network interfaces (PF/VF), bypassing the kernel network stack.
