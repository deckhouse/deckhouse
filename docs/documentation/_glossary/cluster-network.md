---
title: Cluster network
description: An additional software-defined network that is publicly available in every project of the cluster and is configured and managed by the administrator.
lang: en
---

Examples of a cluster network are a public WAN network or a shared network for traffic exchange between [projects](glossary.html#project). The administrator creates it with a [ClusterNetwork](/modules/sdn/cr.html#clusternetwork) object of the [`sdn`](/modules/sdn/) module and, if needed, enables [IPAM](glossary.html#sdn-ipam) for it with a cluster-wide address pool. To use such a network in application pods, contact the cluster administrator. The other kind of [additional network](glossary.html#additional-network) is the [project network](glossary.html#project-network).

For details, see [Software-defined networking (SDN)](admin/configuration/network/sdn/).
