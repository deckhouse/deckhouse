---
title: Worker node
description: A node intended to run user applications. Its resources are dictated by the workload, with a part of them reserved for kubelet and system pods.
lang: en
---

Worker nodes are present in every [deployment option](glossary.html#deployment-scenario) except the single-node cluster, where the only node takes their role. In a single-master cluster, some Deckhouse Platform components also run on worker nodes. The actual node size depends on the applications running on it.

For details, see the [Resource requirements](requirements/#resource-requirements) section of the "System requirements" page.
