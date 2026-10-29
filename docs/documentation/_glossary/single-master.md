---
title: Single-master cluster
description: A control plane operation mode with one master node. kube-apiserver uses only the local etcd instance and listens only on the address of the master node.
lang: en
---

A proxy server on the node handles requests on `localhost`. When master nodes are added or removed, Deckhouse Platform migrates the [control plane](glossary.html#control-plane) between the single-master and [multi-master](glossary.html#multi-master) modes.

For details, see [Scaling and changing master nodes](admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html).
