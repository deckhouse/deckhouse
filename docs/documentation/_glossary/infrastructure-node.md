---
title: Infrastructure node
description: A collective name for the master, frontend, system and monitoring nodes of a cluster, as opposed to worker nodes that run user applications.
lang: en
---

Infrastructure nodes run the cluster control plane and Deckhouse Platform components. The recommended resource minimums depend on the role: [master nodes](glossary.html#master-node) and [monitoring nodes](glossary.html#monitoring-node) need fast disks, [frontend nodes](glossary.html#frontend-node) need the least resources, and the size of [system nodes](glossary.html#system-node) depends on whether the cluster has dedicated monitoring nodes.

For details, see [Going to production](guides/production.html).
