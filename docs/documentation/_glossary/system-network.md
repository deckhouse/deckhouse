---
title: System network
description: An additional service network on the cluster nodes, created by the [`sdn`](/modules/sdn/) module on top of an existing underlay network for node-level service traffic.
lang: en
---

A system network is created with a [SystemNetwork](/modules/sdn/cr.html#systemnetwork) object. Interfaces connected to a system network get cluster-scoped addresses through [ClusterIPAddress](/modules/sdn/cr.html#clusteripaddress), like interfaces of [cluster networks](glossary.html#cluster-network). It belongs to the node-level functions of [software-defined networks](glossary.html#sdn), together with the configuration of node interfaces and [underlay networks](glossary.html#underlay-network).

For details, see [Software-defined networking (SDN)](admin/configuration/network/sdn/).
