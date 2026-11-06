---
title: Arbiter node
description: A node that holds only etcd, without the other control plane components, to provide the etcd quorum in HA mode with two master nodes.
lang: en
---

[HA mode](glossary.html#ha-mode) with two [master nodes](glossary.html#master-node) and an arbiter node meets the high availability requirements with limited resources. The arbiter node must have the label `node.deckhouse.io/etcd-arbiter: ""` and a taint that keeps user workloads off it. It needs at least 2 CPU cores, 4 GB of RAM and 8 GB of disk space for etcd, and the same network latency as the master nodes.

For details, see [Managing HA mode](admin/configuration/high-reliability-and-availability/enable.html).
