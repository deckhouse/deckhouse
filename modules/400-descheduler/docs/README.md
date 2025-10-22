---
title: "The descheduler module"
description: "Deckhouse Kubernetes Platform, the descheduler module. Every 15 minutes, analyzes the cluster state and performs pod eviction according to the conditions described in the active strategies."
---

Every 15 minutes, the module analyzes the cluster state and performs pod eviction according to the conditions described in the active [strategies](#strategies). Evicted pods go through the scheduling process again, considering the current state of the cluster. This helps redistribute workloads according to the chosen strategy.
 
The module is based on the [descheduler](https://github.com/kubernetes-sigs/descheduler) project.

## Features of the module

* The module can take into account the pod priority class (parameter [spec.priorityClassThreshold](cr.html#descheduler-v1alpha2-spec-priorityclassthreshold)), restricting its operation to only those pods that have a priority class lower than the specified threshold;
* The module does not evict pods in the following cases:
  * a pod is in the `d8-*` or `kube-system` namespaces;
  * a pod has a `priorityClassName` `system-cluster-critical` or `system-node-critical`;
  * a pod is associated with a local storage;
  * a pod is associated with a [DaemonSet](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/);
  * pod eviction will violate [Pod Disruption Budget (PDB)](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/);
  * there are no available nodes to run the evicted pod.
* Pods with the `Best effort` priority class are evicted before those with `Burstable` and `Guaranteed`.

Descheduler uses parameters with the `labelSelector` syntax from Kubernetes to filter pods and nodes:

* `podLabelSelector` — limits pods by labels;
* `namespaceLabelSelector` — filters pods by namespaces;
* `nodeLabelSelector` — selects nodes by labels.

## Strategies

### HighNodeUtilization

{% alert level="info" %}
More compactly places pods. Requires scheduler configuration and enabling auto-scaling.

To use `HighNodeUtilization`, you must explicitly specify the [high-node-utilization](../control-plane-manager/faq.html#scheduler-profiles) scheduler profile for each pod (this profile cannot be set as the default).
{% endalert %}

This strategy identifies *under utilized nodes* and evicts pods from them to redistribute them more compactly across fewer nodes.

**Under utilized node** — A node whose resource usage is below all the threshold values specified in the [strategies.highNodeUtilization.thresholds](cr.html#descheduler-v1alpha2-spec-strategies-highnodeutilization-thresholds) section.

The strategy is enabled by the parameter [spec.strategies.highNodeUtilization.enabled](cr.html#descheduler-v1alpha2-spec-strategies-highnodeutilization-enabled).

{% alert level="warning" %}
In GKE, you cannot configure the default scheduler, but you can use the `optimize-utilization` strategy or deploy a second custom scheduler.
{% endalert %}

{% alert level="warning" %}
Node resource usage takes into account [extended resources](https://kubernetes.io/docs/tasks/configure-pod-container/extended-resource/) and is calculated based on pod requests and limits ([requests and limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)), not actual consumption. This approach ensures consistency with the kube-scheduler, which uses a similar principle when scheduling pods on nodes. This means that resource usage metrics displayed by Kubelet (or tools like `kubectl top`) might differ from calculated metrics, as Kubelet and related tools show actual resource consumption.
{% endalert %}

### LowNodeUtilization

{% alert level="info" %}
Loads the nodes more evenly.
{% endalert %}

This strategy identifies *under utilized nodes* and evicts pods from other *over utilized nodes*. The strategy assumes that the evicted pods will be recreated on the under utilized nodes (following normal scheduler behavior).

**Under utilized node** — A node whose resource usage is below all the threshold values specified in the [strategies.lowNodeUtilization.thresholds](cr.html#descheduler-v1alpha2-spec-strategies-lownodeutilization-thresholds) section.

**Over utilized node** — A node whose resource usage exceeds at least one of the threshold values specified in the [strategies.lowNodeUtilization.targetThresholds](cr.html#descheduler-v1alpha2-spec-strategies-lownodeutilization-targetthresholds) section.

Nodes with resource usage in the range between `thresholds` and `targetThresholds` are considered optimally utilized. Pods on these nodes will not be evicted.

The strategy is enabled by the parameter [spec.strategies.lowNodeUtilization.enabled](cr.html#descheduler-v1alpha2-spec-strategies-lownodeutilization-enabled).

{% alert level="warning" %}
Node resource usage takes into account [extended resources](https://kubernetes.io/docs/tasks/configure-pod-container/extended-resource/) and is calculated based on pod requests and limits ([requests and limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)), not actual consumption. This approach ensures consistency with the kube-scheduler, which uses a similar principle when scheduling pods on nodes. This means that resource usage metrics displayed by Kubelet (or tools like `kubectl top`) might differ from calculated metrics, as Kubelet and related tools show actual resource consumption.
{% endalert %}

### RemoveDuplicates

{% alert level="info" %}
Prevents multiple pods from the same controller (ReplicaSet, ReplicationController, StatefulSet) or the same Job from running on the same node.
{% endalert %}

The strategy ensures that no more than one pod of a ReplicaSet, ReplicationController, StatefulSet, or pods of a single Job is running on the same node. If there are two or more such pods, the module evicts the excess pods so that they are better distributed across the cluster.

The situation can occur if some nodes in the cluster have failed for any reason, and the pods from those nodes have been moved to other nodes. Once the failed nodes become available again to accept load, this strategy can be used to evict duplicate pods from other nodes.

The strategy is enabled by the parameter [strategies.removeDuplicates.enabled](cr.html#descheduler-v1alpha2-spec-strategies-removeduplicates-enabled).

### RemovePodsViolatingInterPodAntiAffinity

{% alert level="info" %}
Evicts pods violating [inter-pod affinity and anti-affinity rules](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity) to ensure compliance.
{% endalert %}

The strategy ensures that pods violating [inter-pod affinity and anti-affinity rules](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity) are evicted from nodes.

For example, if there is **podA** on a node and **podB** and **podC** (running on the same node) have anti-affinity rules which prohibit them to run on the same node, then **podA** will be evicted from the node so that **podB** and **podC** could run. This issue could happen, when the anti-affinity rules for **podB** and **podC** are created when they are already running on node.

The strategy is enabled by the parameter [spec.strategies.highNodeUtilization.enabled](cr.html#descheduler-v1alpha2-spec-strategies-highnodeutilization-enabled).

### RemovePodsViolatingNodeAffinity

{% alert level="info" %}
Evicts pods violating [node affinity rules](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity) to ensure compliance.
{% endalert %}

The strategy makes sure all pods violating [node affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity) are eventually removed from nodes.

Essentially, depending on the settings of the parameter [strategies.removePodsViolatingNodeAffinity.nodeAffinityType](cr.html#descheduler-v1alpha2-spec-strategies-removepodsviolatingnodeaffinity-nodeaffinitytype), the strategy temporarily implement the rule `requiredDuringSchedulingIgnoredDuringExecution` of the pod's node affinity as the rule `requiredDuringSchedulingRequiredDuringExecution`, and the rule `preferredDuringSchedulingIgnoredDuringExecution` as the rule `preferredDuringSchedulingPreferredDuringExecution`.

Example for `nodeAffinityType: requiredDuringSchedulingIgnoredDuringExecution`. There is a pod scheduled to a node which satisfies the node affinity rule `requiredDuringSchedulingIgnoredDuringExecution` at the time of scheduling. If over time this node no longer satisfies the node affinity rule, and there is another node available that satisfies the node affinity rule, the strategy evicts the pod from the node it was originally scheduled to.

Example for `nodeAffinityType: preferredDuringSchedulingIgnoredDuringExecution`. There is a pod scheduled to a node because at the time of scheduling there were no other nodes that satisfied the node affinity rule `preferredDuringSchedulingIgnoredDuringExecution`. If over time an available node that satisfies this rule appears in the cluster, the strategy evicts the pod from the node it was originally scheduled to.

The strategy is enabled by the parameter [strategies.removePodsViolatingNodeAffinity.enabled](cr.html#descheduler-v1alpha2-spec-strategies-removepodsviolatingnodeaffinity-enabled).
