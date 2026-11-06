---
title: Worker node
description: A node intended to run user applications. Its resources are dictated by the workload, with at least 1 CPU and 2 GB of memory reserved for kubelet and system pods.
lang: en
---

Worker nodes are the only node role that the [deployment scenarios](glossary.html#deployment-scenario) require in every configuration. In the minimum configuration some Deckhouse Platform components also run on a worker node. The resource minimums are similar to those of a [master node](glossary.html#master-node), but the actual size depends on the applications running on the node.

For details, see [Hardware requirements](guides/hardware-requirements.html).
