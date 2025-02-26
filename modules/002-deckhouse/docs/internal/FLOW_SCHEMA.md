# Flow Schema

The module deploys [FlowSchema and PriorityLevelConfiguration](https://kubernetes.io/docs/concepts/cluster-administration/flow-control/) to prevent API overloading.

`FlowSchema` sets `PriorityLevel` for `list` requests from all service accounts in Deckhouse namespaces (having label `heritage: deckhouse`) to the following apiGroups:
* `v1` (Pods, Secrets, ConfigMaps, Nodes, etc.). This helps in the case of many core resources in the cluster (for example, Secrets or Pods).
* `apps/v1` (DaemonSets, Deployments, StatefulSets, ReplicaSets, etc.). This helps in the case of many deployed applications in the cluster (for example, Deployments).
* `deckhouse.io` (Deckhouse custom resources). This helps in the case of many various deckhouse CRs in the cluster.
* `cilium.io` (cilium custom resources). This helps in the case of many cilium policies in the cluster.

All API requests corresponding to `FlowSchema` are placed into the same queue.
