---
title: Multi-master cluster
description: A control plane operation mode with several master nodes. kube-apiserver interacts with all etcd instances, and requests are forwarded to other nodes if the local kube-apiserver is unavailable.
lang: en
---

The multi-master mode provides high availability of the [control plane](glossary.html#control-plane) and supports scaling of [master nodes](glossary.html#master-node). When the cluster has more than one master node, [HA mode](glossary.html#ha-mode) is enabled automatically. In a configuration with two master nodes, an [arbiter node](glossary.html#arbiter-node) provides the etcd quorum.

For details, see [Scaling and changing master nodes](admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html).
