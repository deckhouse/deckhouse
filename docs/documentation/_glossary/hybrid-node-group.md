---
title: Hybrid node group
description: A node group that includes both Static nodes deployed in the cloud and servers in the on-premises data centre, bare metal or virtual machines.
lang: en
---

In a hybrid [node group](glossary.html#node-group) the primary workload can run on bare-metal servers while cloud instances serve as a scalable addition during peak loads. All nodes are [static nodes](glossary.html#static-node), so the [`node-manager`](/modules/node-manager/) module works as for static nodes. The only requirement is L3 connectivity between the data centre and the cloud. A hybrid node group differs from a [hybrid cluster](glossary.html#hybrid-cluster), which combines static nodes with [CloudEphemeral nodes](glossary.html#cloud-ephemeral-node).

For details, see [Hybrid node groups and clusters](architecture/cluster-and-infrastructure/node-management/hybrid-nodegroups-and-clusters.html).
