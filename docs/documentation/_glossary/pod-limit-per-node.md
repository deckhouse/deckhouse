---
title: Pod limit per node
description: The maximum number of pods that can be placed on a node. In Deckhouse Platform it is derived from the node subnet size set by the podSubnetNodeCIDRPrefix parameter when the cluster is deployed.
lang: en
---

With a prefix of 24 or larger a node holds 120 pods, with 23 it holds 250, with 22 it holds 500, and with 21 or smaller it holds 1000. The parameter is part of the [protected parameters](glossary.html#protected-parameters) of the cluster, so choose the value at deployment time if many pods per node are expected. The control plane components for high pod density are configured by Deckhouse Platform automatically.

For details, see [High pod density](guides/high-pod-density.html).
