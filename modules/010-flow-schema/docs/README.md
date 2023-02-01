---
title: "The flow-schema module"
---

This module deploys [FlowSchema and PriorityLevelConfiguration](https://kubernetes.io/docs/concepts/cluster-administration/flow-control/) to prevent API overloading.


`FlowSchema` sets `PriorityLevel` for `list` requests from all service accounts in Deckhouse namespaces (having label `heritage: deckhouse`) to:
* `v1` apigroup (Pods, Secrets, ConfigMaps, Nodes, etc.). This helps in case of a large number of core resources in the cluster (for example, secrets or pods).
* `deckhouse.io` apigroup (Deckhouse custom resources). This helps in case of a large number various deckhouse CRs in the cluster.
* `cilium.io` apigroup (cilium custom resources). This helps in case of a large number of cilium policies in the cluster.

All requests to the API corresponding to `FlowSchema` are placed into the same queue.
