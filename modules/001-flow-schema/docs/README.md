---
title: "The flow-schema module"
---

This module deploys [FlowSchema and PriorityLevelConfiguration](https://kubernetes.io/docs/concepts/cluster-administration/flow-control/) to prevent API overloading.


`FlowSchema` sets `PriorityLevel` for `list` requests from all service accounts in Deckhouse namespaces (having label `heritage: deckhouse`) to:
* `v1` apigroup (Pods, Secrets, ConfigMaps, Nodes, etc.). This helps in case of big amount of core resources in cluster (for example, secrets or pods).
* `deckhouse.io` apigroup (Deckhouse custom resources). This helps in case of big amount various deckhouse CRs in cluster.
* `cilium.io` apigroup (cilium custom resources). This helps in case of big amount of cilium policies in cluster.

All requests to the API corresponding to `FlowSchema` are placed into the same queue.
