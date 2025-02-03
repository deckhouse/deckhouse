---
title: "Scheduler"
permalink: en/virtualization-platform/documentation/admin/platform-management/control-plane-settings/scheduler.html
---

## Description of the scheduler algorithm

The Kubernetes scheduler (the `kube-scheduler` component) is responsible for distributing pods across nodes.

The scheduler's decision-making algorithm is divided into 2 phases: `Filtering` and `Scoring`.

Within each phase, the scheduler launches a set of plugins that implement decision-making, for example:

- **ImageLocality** — the plugin gives preference to nodes that already have container images that are used in the pod being launched. Phase: `Scoring`.
- **TaintToleration** — implements the [taints and tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) mechanism. Phases: `Filtering, Scoring`.
- **NodePorts** — checks whether the node has free ports required to launch the pod. Phase: `Filtering`.

The full list of plugins can be found in the [Kubernetes documentation](https://kubernetes.io/docs/reference/scheduling/config/#scheduling-plugins).

The first phase of filtering (`Filtering`) uses filter plugins that check nodes for matching filter conditions (taints, nodePorts, nodeName, unschedulable, etc.).

The filtered list is sorted by alternating zones to avoid placing all pods in one zone. Let's assume that after filtering, the nodes remaining are distributed among zones as follows:

```text
Zone 1: Node 1, Node 2, Node 3, Node 4
Zone 2: Node 5, Node 6
```

In this case, they will be selected in the following order:

```text
Node 1, Node 5, Node 2, Node 6, Node 3, Node 4
```

Note that for optimization purposes, not all nodes that meet the conditions are selected, but only a part of them. By default, the node number selection function is linear. For a cluster of ≤50 nodes, 100% of the nodes will be selected,
for a cluster of 100 nodes - 50%, and for a cluster of 5000 nodes - 10%. The minimum value is 5% for nodes greater
than 5000. For more information on node limitation, see the Kubernetes documentation for the [KubeSchedulerConfiguration](https://kubernetes.io/docs/reference/config-api/kube-scheduler-config.v1/#kubescheduler-config-k8s-io-v1-KubeSchedulerConfiguration) resource.
Deckhouse uses the default value, so in very large clusters, you need to take this scheduler behavior into account.

Once the nodes that match the filter conditions are selected, the `Scoring` phase is launched. The plugins of this phase analyze the list of filtered nodes and assign a score to each node. Scores from different plugins are summed up. This phase evaluates available resources on nodes, pod capacity, affinity, volume provisioning, etc.

The result of this phase is a list of nodes with the highest score. If there is more than one node in the list, the node is chosen randomly.

### Documentation

- [General description of scheduler](https://kubernetes.io/docs/concepts/scheduling-eviction/kube-scheduler/).
- [Plugin system](https://kubernetes.io/docs/reference/scheduling/config/#scheduling-plugins).
- [Details of node filtering](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduler-perf-tuning/).
- [Source code of scheduler](https://github.com/kubernetes/kubernetes/tree/master/cmd/kube-scheduler).

## Changing and extending the scheduler logic

You can use the [extension plugin mechanism](https://github.com/kubernetes/enhancements/blob/master/keps/sig-scheduling/624-scheduling-framework/README.md) to change the scheduler logic.

Each plugin is a webhook that meets the following requirements:

* Using TLS.
* Availability through a service within the cluster.
* Support for standard *Verbs* (filterVerb = filter, prioritizeVerb = prioritize).
* It is also assumed that all added plugins can cache node information (`nodeCacheCapable: true`).

You can connect such an extender webhook using the [KubeSchedulerWebhookConfiguration](../../../../reference/cr/kubeschedulerwebhookconfiguration.html) resource.

{% alert level="danger" %}
When using the `failurePolicy: Fail` option, an error in the webhook operation causes the scheduler to stop working and new pods will not be able to start.
{% endalert %}

## Speeding up recovery when a node is lost

<!-- TODO here we need to somehow connect it with virtual machines. -->

By default, if a node does not report its status within 40 seconds, it is marked as unavailable. After another 5 minutes, the pods of such a node will be assigned by the scheduler to other nodes. The total time of application unavailability is about 6 minutes.

For specific tasks, when an application cannot be launched in multiple instances, there is a way to reduce the unavailability period in the `control-plane-manager` module settings:

1. Reduce the time it takes for a node to transition to the `Unreachable` state when communication with it is lost by setting the `nodeMonitorGracePeriodSeconds` parameter.
1. Reduce the pod node reassignment timeout in the `failedNodePodEvictionTimeoutSeconds` parameter.

### Example

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
name: control-plane-manager
spec:
version: 1
settings:
nodeMonitorGracePeriodSeconds: 10
failedNodePodEvictionTimeoutSeconds: 50
```

In this case, if the connection with the node is lost, applications will be launched on other nodes in about 1 minute.

{% alert level="warning" %}
Both of the described parameters have a direct impact on the consumption of CPU and memory resources on master nodes. Reduced timeouts force system components to send statuses and check resource states more often.

When selecting suitable values, pay attention to the resource consumption graphs of master nodes. Be prepared for the fact that in order to ensure acceptable parameter values, it may be necessary to increase the capacity allocated to master nodes.
{% endalert %}
