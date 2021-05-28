---
title: "Модуль descheduler"
---

## Description
### Background

Kube-scheduler is the Kubernetes component that is responsible for selecting optimal nodes for pods to run on. After finding a feasible node for a pod, it notifies the Kubernetes API (the process is known as binding). Kube-scheduler matches a pod to a node based on policies specified for a pod and node status. The scheduling decision is made when a new pod gets the pending status. Since a Kubernetes cluster is very dynamic and its state changes over time, you may need to relocate some running pod to another node for various reasons:

* Some cluster nodes are underutilized, while others are overutilized;
* Initial conditions on which the scheduling decision was based are no longer relevant (some taints/labels were added or deleted, pod/node affinity was modified);
* Some nodes are no longer members of the cluster, and their pods have been rescheduled to other nodes;
* New nodes are added to the cluster.

The descheduler discovers pods based on policies and evicts the non-relevant ones. Then kube-scheduler schedules these pods based on the new conditions.

### The process

This module adds a [descheduler](https://github.com/kubernetes-incubator/descheduler) Deployment to the cluster. It runs every 15 minutes, finds the non-optimal pods using the [config-map](templates/config-map.yaml), and evicts them.

The descheduler has 8 strategies built-in:
* RemoveDuplicates (**disabled by default**)
* LowNodeUtilization (**disabled by default**)
* RemovePodsViolatingInterPodAntiAffinity (**enabled by default**)
* RemovePodsViolatingNodeAffinity (**enabled by default**)
* RemovePodsViolatingNodeTaints (**disabled by default**)
* RemovePodsViolatingTopologySpreadConstraint (**disabled by default**)
* RemovePodsHavingTooManyRestarts (**disabled by default**)
* PodLifeTime (**disabled by default**)

#### RemoveDuplicates

This strategy makes sure that no more than one pod of the same controller (rs, rc, deploy, job) is running on the same node. If there are two such pods on one node, the descheduler kills one of them.

Suppose there are three nodes (say, the first node bears the greater load than the other two), and we want to deploy six application replicas. In this case, the scheduler will schedule 0 or 1 pod to that overutilized node, while other replicas will be distributed between two other nodes. Thus, the descheduler will be killing "extra" pods on those two nodes every 15 minutes, hoping that the scheduler will bind those pods to the first node.

#### LowNodeUtilization

The descheduler finds underutilized or overutilized nodes using cpu/memory/pods (in %) thresholds and evict pods from overutilized nodes hoping that these pods will be rescheduled on underutilized nodes. Note that this strategy takes into account pod requests instead of actual resources consumed.
The thresholds for identifying underutilized or overutilized nodes are currently preset and cannot be changed:
* Criteria to identify underutilized nodes:
  * cpu — 40%
  * memory — 50%
  * pods — 40%
* Criteria to identify overutilized nodes:
  * cpu — 80%
  * memory — 90%
  * pods — 80%

#### RemovePodsViolatingInterPodAntiAffinity

This strategy ensures that pods violating inter-pod anti-affinity are removed from nodes. We find it hard to imagine a situation when inter-pod anti-affinity can be violated, while the official descheduler documentation does not provide much guidance either:

> This strategy makes sure that pods violating interpod anti-affinity are removed from nodes. For example, if there is podA on node and podB and podC(running on same node) have antiaffinity rules which prohibit them to run on the same node, then podA will be evicted from the node so that podB and podC could run. This issue could happen, when the anti-affinity rules for pods B,C are created when they are already running on node.

#### RemovePodsViolatingNodeAffinity

This strategy removes a pod from a node if the latter no longer satisfies a pod's affinity rule (`requiredDuringSchedulingIgnoredDuringExecution`). The descheduler notices that and evicts the pod if another node is available that satisfies the affinity rule.

#### RemovePodsViolatingNodeTaints
This strategy evicts pods violating NoSchedule taints on nodes. Suppose a pod set to tolerate some taint is running on a node with this taint. If the node’s taint is updated or removed, the pod will be evicted.

#### RemovePodsViolatingTopologySpreadConstraint
This strategy ensures that pods violating the [Pod Topology Spread Constraints](https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/) will be evicted from nodes.

#### RemovePodsHavingTooManyRestarts
This strategy ensures that pods having over a hundred container restarts (including init-containers) are removed from nodes.

#### PodLifeTime
This strategy evicts pods that are Pending for more than 24 hours.

### Known nuances

* The descheduler does not evict critical pods (with priorityClassName set to `system-cluster-critical` or `system-node-critical`).
* It takes into account the priorityClass when evicting pods from a high-loaded node.
* The descheduler does not evict pods that are associated with a DaemonSet or aren't covered by a controller.
* It never evicts pods with local storage enabled.
* The Best effort pods are evicted before Burstable and Guaranteed ones.
* The descheduler uses the Evict API and therefore takes into account the [Pod Disruption Budget](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/). The pod will not be evicted if descheduling violates the PDB.

### An example of the configuration

```yaml
descheduler: |
  removePodsViolatingNodeAffinity: false
  removeDuplicates: true
  lowNodeUtilization: true
  nodeSelector:
    node-role/example: ""
  tolerations:
  - key: dedicated
    operator: Equal
    value: example
```
