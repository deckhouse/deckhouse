---
title: "The descheduler module"
---

The module runs a [descheduler](https://github.com/kubernetes-sigs/descheduler) with [strategies](#strategies) defined in a `Descheduler` custom resource.

descheduler every 15 minutes evicts Pods that satisfy strategies enabled in the `Descheduler` custom resource. This leads to forced run the scheduling process for evicted Pods.

## Nuances of descheduler operation

* descheduler do not take into account pods in `d8-*` and `kube-system` namespaces;
* Pods with local storage enabled are never evicts;
* Pods that are associated with a DaemonSet are never evicts;
* Pods with [priorityClassName](../001-priority-class/) set to `system-cluster-critical` or `system-node-critical` (*critical* Pods) are never evicts;
* descheduler takes into account the priority class when evicting Pods from a high-loaded node (check out the [priority-class](../001-priority-class/) module);
* The Best effort Pods are evicted before Burstable and Guaranteed ones;
* descheduler takes into account the [Pod Disruption Budget](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/): the Pod will not be evicted if descheduling violates the PDB;
* descheduler takes into account node fitting. If no nodes available to start evicted pod, pod is not evicted.

To limit pods set, `labelSelector` parameter is used.
To set up node fit list, `nodeSelector` parameter is used. `nodeSelector` has the same syntax as `labelSelector`. Node fit list always excludes nodes with labels `node.deckhouse.io/group: master` and `node.deckhouse.io/group: system`.

## Strategies

### HighNodeUtilization

This strategy finds nodes that are under utilized and evicts pods from the nodes in the hope that these pods will be scheduled compactly into fewer nodes. Used in conjunction with node auto-scaling, this strategy is intended to help trigger down scaling of under utilized nodes. This strategy must be used with the scheduler scoring strategy MostAllocated.
The under utilization of nodes is determined by a configurable threshold `thresholds`. The threshold `thresholds` can be configured for cpu, memory, number of pods, and extended resources in terms of percentage. The percentage is calculated as the current resources requested on the node vs total allocatable. For pods, this means the number of pods on the node as a fraction of the pod capacity set for that node.
If a node's usage is below threshold for all (cpu, memory, number of pods and extended resources), the node is considered underutilized. Currently, pods request resource requirements are considered for computing node resource utilization. Any node above `thresholds` is considered appropriately utilized and is not considered for eviction.
The `thresholds` parameter could be tuned as per your cluster requirements. Note that this strategy evicts pods from underutilized nodes (those with usage below `thresholds`) so that they can be recreated in appropriately utilized nodes. The strategy will abort if any number of underutilized nodes or appropriately utilized nodes is zero.

NOTE: Node resource consumption is determined by the requests and limits of pods, not actual usage. This approach is chosen in order to maintain consistency with the kube-scheduler, which follows the same design for scheduling pods onto nodes. This means that resource usage as reported by Kubelet (or commands like kubectl top) may differ from the calculated consumption, due to these components reporting actual usage metrics.

### LowNodeUtilization

This strategy finds nodes that are under utilized and evicts pods, if possible, from other nodes in the hope that recreation of evicted pods will be scheduled on these underutilized nodes.
The under utilization of nodes is determined by a configurable threshold `thresholds`. The threshold `thresholds` can be configured for cpu, memory, number of pods, and extended resources in terms of percentage (the percentage is calculated as the current resources requested on the node vs total allocatable. For pods, this means the number of pods on the node as a fraction of the pod capacity set for that node).
If a node's usage is below threshold for all (cpu, memory, number of pods and extended resources), the node is considered underutilized. Currently, pods request resource requirements are considered for computing node resource utilization.
There is another configurable threshold, `targetThresholds`, that is used to compute those potential nodes from where pods could be evicted. If a node's usage is above targetThreshold for any (cpu, memory, number of pods, or extended resources), the node is considered over utilized. Any node between the thresholds, `thresholds` and `targetThresholds` is considered appropriately utilized and is not considered for eviction. The threshold, `targetThresholds`, can be configured for cpu, memory, and number of pods too in terms of percentage.
These thresholds, `thresholds` and `targetThresholds`, could be tuned as per your cluster requirements. Note that this strategy evicts pods from overutilized nodes (those with usage above `targetThresholds`) to underutilized nodes (those with usage below `thresholds`), it will abort if any number of underutilized nodes or overutilized nodes is zero.

NOTE: Node resource consumption is determined by the requests and limits of pods, not actual usage. This approach is chosen in order to maintain consistency with the kube-scheduler, which follows the same design for scheduling pods onto nodes. This means that resource usage as reported by Kubelet (or commands like kubectl top) may differ from the calculated consumption, due to these components reporting actual usage metrics.
