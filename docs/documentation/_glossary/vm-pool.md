---
title: Virtual machine pool
description: A set of identical virtual machines, replicas created from one template, whose number Deckhouse Platform maintains. A pool can be scaled through the scale subresource, HPA or KEDA. Available in commercial editions.
lang: en
---

A pool is described by [VirtualMachinePool](/modules/virtualization/cr.html#virtualmachinepool). Its template matches a regular virtual machine specification, so a replica is no different from a manually created [virtual machine](glossary.html#virtual-machine). Replica disks are described once in disk templates with a [reclaim policy](glossary.html#vm-pool-reclaim-policy), a size and a data source, and the template references them in the boot order. Shared [images](glossary.html#vm-image), such as one ISO for all replicas, are attached read-only.

Replicas are differentiated by [provisioning](glossary.html#vm-provisioning), so the `Legacy` OS type is not supported in a pool.

For details, see [Virtual machine pools](user/virtualization/vm-pools.html).
