---
title: Quorum
description: The majority of the members of a distributed system (more than half of their total number) that must be available for the system to keep working and accept changes.
lang: en
---

In Deckhouse Platform, the quorum matters most for etcd, the storage of the [control plane](glossary.html#control-plane). The etcd cluster accepts changes only while the majority of its members (N/2 + 1) are available: a cluster of three members tolerates the failure of one member, a cluster of five tolerates the failure of two. An even number of members does not improve fault tolerance: with two members, the failure of either one loses the quorum. For this reason, use an odd number of [master nodes](glossary.html#master-node), usually three. In a configuration with two master nodes, the third etcd member may run on an [arbiter node](glossary.html#arbiter-node), which provides the quorum.

For details, see [High availability](admin/configuration/platform-scaling/control-plane/control-plane-recovery-and-debugging.html#high-availability) in the control plane recovery guide.
