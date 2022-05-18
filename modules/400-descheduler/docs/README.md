---
title: "The descheduler module"
---

The module runs a [descheduler](https://github.com/kubernetes-incubator/descheduler) with **predefined** [strategies](#strategies) in a cluster.

descheduler every 15 minutes evicts Pods that satisfy strategies enabled in the [module configuration](configuration.html). This leads to forced run the scheduling process for evicted Pods.

## Nuances of descheduler operation

* descheduler takes into account the priorityClass when evicting Pods from a high-loaded node (check out the [priority-class](../001-priority-class/) module);
* Pods with [priorityClassName](../001-priority-class/) set to `system-cluster-critical` or `system-node-critical` (*critical* Pods) are never evicts;
* Pods that are associated with a DaemonSet or aren't covered by a controller are never evicts;
* Pods with local storage enabled are never evicts;
* The Best effort Pods are evicted before Burstable and Guaranteed ones;
* descheduler takes into account the [Pod Disruption Budget](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/):  the Pod will not be evicted if descheduling violates the PDB.

## Strategies

You can enable or disable a strategy in the [module configuration](configuration.html).

The following strategies are **enabled** by default:
* [RemovePodsViolatingInterPodAntiAffinity](#removepodsviolatinginterpodantiaffinity)
* [RemovePodsViolatingNodeAffinity](#removepodsviolatingnodeaffinity)

### HighNodeUtilization

This strategy finds nodes that are under utilized and evicts Pods in the hope that these Pods will be scheduled
compactly into fewer nodes. This strategy must be used with the scheduler strategy `MostRequestedPriority`.

The thresholds for identifying underutilized nodes are currently preset and cannot be changed:
* Criteria to identify underutilized nodes:
  * CPU — 50%
  * memory — 50%

### LowNodeUtilization

The descheduler finds underutilized or overutilized nodes using cpu/memory/Pods (in %) thresholds and evict Pods from overutilized nodes hoping that these Pods will be rescheduled on underutilized nodes. Note that this strategy takes into account Pod requests instead of actual resources consumed.

The thresholds for identifying underutilized or overutilized nodes are currently preset and cannot be changed:
* Criteria to identify underutilized nodes:
  * CPU — 40%
  * memory — 50%
  * Pods — 40%
* Criteria to identify overutilized nodes:
  * CPU — 80%
  * memory — 90%
  * Pods — 80%

### PodLifeTime

This strategy evicts Pods that are Pending for more than 24 hours.

### RemoveDuplicates

This strategy makes sure that no more than one Pod of the same controller (RS, RC, Deploy, Job) is running on the same node. If there are two such Pods on one node, the descheduler kills one of them.

Suppose there are three nodes (say, the first node bears the greater load than the other two), and we want to deploy six application replicas. In this case, the scheduler will schedule 0 or 1 Pod to that overutilized node, while other replicas will be distributed between two other nodes. Thus, the descheduler will be killing "extra" Pods on those two nodes every 15 minutes, hoping that the scheduler will bind those Pods to the first node.

### RemovePodsHavingTooManyRestarts

This strategy ensures that Pods having over a hundred container restarts (including init-containers) are removed from nodes.

### RemovePodsViolatingInterPodAntiAffinity

This strategy ensures that Pods violating inter-pod anti-affinity are removed from nodes. We find it hard to imagine a situation when inter-pod anti-affinity can be violated, while the official descheduler documentation does not provide much guidance either:

> This strategy makes sure that Pods violating inter-pod anti-affinity are removed from nodes. For example, if there is podA on node and podB and podC (running on same node) have anti-affinity rules which prohibit them to run on the same node, then podA will be evicted from the node so that podB and podC could run. This issue could happen, when the anti-affinity rules for Pods B, C are created when they are already running on node.

### RemovePodsViolatingNodeAffinity

This strategy removes a Pod from a node if the latter no longer satisfies a Pod's affinity rule (`requiredDuringSchedulingIgnoredDuringExecution`). The descheduler notices that and evicts the Pod if another node is available that satisfies the affinity rule.

### RemovePodsViolatingNodeTaints

This strategy evicts Pods violating NoSchedule taints on nodes. Suppose a Pod set to tolerate some taint is running on a node with this taint. If the node’s taint is updated or removed, the Pod will be evicted.

### RemovePodsViolatingTopologySpreadConstraint

This strategy ensures that Pods violating the [Pod Topology Spread Constraints](https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/) will be evicted from nodes.
