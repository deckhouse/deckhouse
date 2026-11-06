---
title: Underlay network
description: Direct connection of physical functions (PF) and virtual functions (VF) of the network interfaces of a node to pods through Kubernetes Dynamic Resource Allocation. It serves workloads that need direct access to hardware, such as DPDK applications.
lang: en
---

Physical network interfaces are connected to pods in one of two modes: `Shared`, where Virtual Functions are created from a Physical Function with SR-IOV and several pods share the hardware, or `Dedicated`, where a pod gets the whole Physical Function. The administrator creates an [UnderlayNetwork](/modules/sdn/cr.html#underlaynetwork) object of the [`sdn`](/modules/sdn/) module and marks the [namespace](glossary.html#namespace) that may use it. The pod requests a device with an annotation.

DPDK, Data Plane Development Kit, is a set of libraries for processing network packets in user space, bypassing the kernel network stack.

For details, see [Connecting physical network interfaces to DPDK application pods](user/network/sdn/underlay.html).
