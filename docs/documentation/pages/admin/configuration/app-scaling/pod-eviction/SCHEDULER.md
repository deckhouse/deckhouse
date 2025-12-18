---
title: "Scheduler"
permalink: en/admin/configuration/app-scaling/pod-eviction/scheduler.html
description: "Configure Kubernetes scheduler in Deckhouse Kubernetes Platform. Pod scheduling policies, node selection, resource allocation, and cluster workload distribution optimization."
---

## Pod scheduling

### General overview of the scheduler

The scheduler is a component responsible for assigning pods to cluster nodes based on available resources and specified rules. It selects the most suitable node for running each pod, considering many factors such as CPU and memory availability, data locality, network topology, node labels, and application-specific requirements.

The scheduler operates in multiple stages, each involving different plugins. These plugins evaluate nodes against specific criteria and help determine the optimal placement for a pod.

Main tasks of the scheduler:

- Filtering — determines which nodes meet the pod’s requirements.
- Scoring — assigns scores to nodes based on various criteria (the higher the score, the better the node).
- Binding — assigns the pod to the best node based on scoring results.

Plugins may participate in one or multiple stages. For example, one plugin might be used only during filtering, while another can be involved in both filtering and scoring.

Examples of plugins:

- ImageLocality (phase: Scoring) — prefers nodes that already have the required container images. This helps reduce deployment time since images do not need to be pulled from a remote repository.

- TaintToleration (phases: Filtering, Scoring) — implements the taints and tolerations mechanism. It helps avoid placing pods on unsuitable nodes or, conversely, forces placement on specific nodes (e.g., those with dedicated resources).

- NodePorts (phase: Filtering) — checks if the node has free ports required by the pod. This is especially important for services using fixed NodePorts, as a node cannot host multiple pods using the same port.

For the complete list of plugins, refer to the [Kubernetes documentation](https://kubernetes.io/docs/reference/scheduling/config/#scheduling-plugins).

### Pod scheduling phases

The process of assigning pods to nodes in DKP involves several key phases. The scheduler analyzes available nodes, applies various filtering and scoring criteria, and then selects the most suitable node to run the pod.

In the Filtering phase, filtering plugins are activated. These plugins evaluate all available nodes and select only those that meet certain conditions (e.g., taints, nodePorts, nodeName, unschedulable, and others).

If the nodes are distributed across different availability zones, DKP alternates between zones during selection to avoid placing all pods in a single zone. For example, if the nodes are distributed like this:

```console
Zone 1: Node 1, Node 2, Node 3, Node 4
Zone 2: Node 5, Node 6
```

The selection will occur in the following order:

```console
Node 1, Node 5, Node 2, Node 6, Node 3, Node 4
```

It is important to note that the scheduler does not evaluate all nodes in the cluster — only a subset is considered to optimize the scheduling process. The number of nodes evaluated depends on the size of the cluster:

- In small clusters (≤50 nodes), all nodes are evaluated.
- In medium clusters (~100 nodes), about 50% of nodes are evaluated.
- In very large clusters (~5000 nodes), only 10% of nodes are evaluated.
- The minimum threshold is 5% of nodes when there are more than 5000.

After the filtering phase, the scoring phase begins. During this stage, each node from the filtered list is scored by various scoring plugins that analyze its characteristics and how well it matches the pod's requirements.

Each plugin evaluates a node based on its specific criteria and assigns it a score. These scores are then aggregated to produce an overall ranking.

Key factors considered during scoring include:

- Node load (`pod capacity`) — available CPU and memory are taken into account. Nodes with more free resources receive higher scores.
- Container image locality (ImageLocality) — nodes that already have the required container images are preferred, as this speeds up pod deployment.
- Affinity and anti-affinity rules — nodes that satisfy required placement (or avoidance) rules based on other pods get higher priority.
- Storage availability (`volume provisioning`) — nodes with the necessary PersistentVolumes receive a higher score.

After all scores are summed, the node with the highest total score is selected for pod placement.

If multiple nodes receive the same highest score, the selection is made randomly to avoid inefficient and predictable load distribution.

### How to modify or extend scheduler logic

The scheduler in DKP can be flexibly configured and extended using custom plugins. This allows its behavior to be adapted to specific cluster requirements, such as considering custom metrics, special workload distribution rules, or node prioritization.

Each such plugin is implemented as a webhook and must meet the following requirements:

- TLS usage – The plugin must be accessed over a secure TLS connection.
- Availability – The plugin must be deployed as a service inside the cluster and accessible via HTTP(S).
- Support for standard verbs – The plugin must support standard scheduler operations:
  - `filterVerb = filter` – participates in node filtering before pod placement.
  - `prioritizeVerb = prioritize` – participates in node scoring during the Scoring phase.
- Node cache capability – It is expected that all extender plugins support caching node data (`nodeCacheCapable: true`) to improve performance.

To connect an extender plugin, use the [KubeSchedulerWebhookConfiguration](/modules/control-plane-manager/cr.html#kubeschedulerwebhookconfiguration) resource. This resource defines the configuration for an external webhook used by the kube-scheduler and enables more advanced scheduling conditions in the cluster, such as:

- Placing application pods closer to data storage nodes,
- Prioritizing nodes based on their current state (e.g., network load or storage subsystem health),
- Segmenting nodes into zones, etc.

Example of connecting an external scheduler plugin via webhook:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: KubeSchedulerWebhookConfiguration
metadata:
  name: sds-replicated-volume
webhooks:
- weight: 5
  failurePolicy: Ignore
  clientConfig:
    service:
      name: scheduler
      namespace: d8-sds-replicated-volume
      port: 8080
      path: /scheduler
    caBundle: ABCD=
  timeoutSeconds: 5
```

{% alert level="danger" %}
If the `failurePolicy: Fail` parameter is used, any failure in the webhook will cause the scheduler to stop functioning entirely, preventing new pods from being scheduled. It is highly recommended to thoroughly test any custom scheduler plugins before deploying them in a production environment.
{% endalert %}

### Additional pod placement mechanisms

Deckhouse Kubernetes Platform provides flexible mechanisms for managing pod placement within a cluster. These mechanisms help optimize load balancing, improve fault tolerance, and separate system and user workloads.

Main placement configuration options:

- Using node labels ([`NodeGroup.spec.nodeTemplate.labels`](/modules/node-manager/cr.html#nodegroup-v1-spec-nodetemplate-labels)).  
  Allows explicitly targeting specific nodes for pods using `spec.nodeSelector` or `spec.affinity.nodeAffinity`.

- Configuring taints and tolerations.  
  Enables limiting pod scheduling to specific nodes, preventing them from being placed on unsuitable hosts.

- Using custom toleration keys ([`settings.modules.placement.customTolerationKeys`](../../../../reference/api/global.html#parameters-modules-placement-customtolerationkeys)).  
  Allows control over the scheduling of critical Deckhouse components (e.g., CNI and CSI) on dedicated nodes.

1. Using `nodeSelector` — allows you to explicitly specify which nodes pods should be scheduled on. This is done by labeling the desired nodes and referencing those labels in the pod’s `spec.nodeSelector`. Example:

   Suppose you have a node named `kube-system-1` that is intended for monitoring services. Label the node:

   ```console
   d8 k label node kube-system-1 node-role/monitoring=""
   ```

   Now, to deploy pods only on this node, add a `nodeSelector` to the Deployment:

   ```yaml
   nodeSelector:
    node-role/monitoring: ""
   ```

1. Using taints and tolerations — unlike `nodeSelector`, taints are used to prevent pods from being scheduled on a node unless they have a matching toleration. This ensures that only specific services run on a given node. Example:

   Suppose you have a node named `kube-frontend-1` that is dedicated exclusively to Ingress controllers. Apply a taint to the node:

   ```console
   d8 k taint node kube-frontend-1 node-role/frontend="":NoExecute
   ```

   Now, only pods with the appropriate toleration will be allowed to run on this node:

   ```yaml
   tolerations:
   - effect: NoExecute
     key: node-role/frontend
   ```

   This mechanism prevents unintended pods from being scheduled on nodes that are meant for specific purposes.

1. Using [`customTolerationKeys`](../../../../reference/api/global.html#parameters-modules-placement-customtolerationkeys) — Deckhouse supports the `customTolerationKeys` mechanism, which explicitly defines allowed toleration keys. This is useful when you need to run system services (such as CNI, CSI, etc.) on specific nodes. Example:

   ```yaml
   customTolerationKeys:
   - dedicated.example.com
   - node-dedicated.example.com/master
   ```

   This mechanism allows:

   - Divide nodes into zones — for example, dedicating some for Ingress controllers and others for system services (Prometheus, VPN, CoreDNS).
   - Isolate critical applications from system components, avoiding resource contention.

### Scheduler profiles

The scheduler supports multiple profiles that define different strategies for distributing pods across nodes in the cluster. Depending on the selected profile, pods will be scheduled with different load balancing logic.

- `default-scheduler` — the default profile that attempts to evenly distribute pods across nodes, preferring less loaded ones.
- `high-node-utilization` — a profile that places pods on more heavily loaded nodes. This can be useful in scenarios where workload consolidation is needed, allowing underutilized nodes to be shut down or repurposed.

To select a scheduler profile, specify it in the `spec.schedulerName` field of the Pod manifest. Example:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: scheduler-example
  labels:
    name: scheduler-example
spec:
  schedulerName: high-node-utilization
  containers:
  - name: example-pod
    image: registry.k8s.io/pause:2.0  
```

## Pod redistribution

DKP analyzes the state of the cluster every 15 minutes and evicts pods that match the conditions described in active scheduling strategies. Evicted pods go through the standard scheduling process again, taking the current cluster state into account. This mechanism allows workloads to be redistributed based on the selected strategy and frees up resources on certain nodes when necessary.

### Considering pod Priority Classes

DKP defines a set of priority classes that determine the importance of pods and the order in which they are evicted during workload redistribution.

If the cluster lacks resources, lower-priority pods may be evicted in favor of higher-priority ones. This ensures that critical services remain operational even when nodes are overloaded.

### How DKP considers pod priority

Deckhouse Kubernetes Platform uses the pod priority mechanism to determine which pods should be evicted when resources are insufficient. The higher the pod’s priority, the lower the chance it will be evicted during redistribution.

This mechanism is controlled by the [`descheduler`](/modules/descheduler/) module, where you can define a priority threshold using the `spec.priorityClassThreshold` parameter. This threshold limits eviction to only those pods with a priority below the specified value.

You can set the threshold in two ways:

- By class name: Use `priorityClassThreshold.name` to evict only pods with a priority lower than the specified [priority class](./priority-classes.html).
- By numeric value: Use `priorityClassThreshold.value` to evict pods with a priority lower than the specified integer value.

Example:

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: Descheduler
metadata:
  name: custom
spec:
  priorityClassThreshold:
    name: high-priority
```

### Which pods are not evicted

- DKP does not evict a pod in the following cases:

  - The pod is in the `d8-*` or `kube-system` namespace;
  - The pod has the `priorityClassName` set to `system-cluster-critical` or `system-node-critical`;
  - The pod is using local storage;
  - The pod is managed by a DaemonSet;
  - Evicting the pod would violate a [Pod Disruption Budget (PDB)](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/);
  - There are no available nodes to reschedule the evicted pod.

If multiple pods match the eviction criteria, DKP applies additional logic:

1. Pods with the lowest priority (`BestEffort`) are evicted first;
1. Then `Burstable` pods are considered;
1. `Guaranteed` pods are evicted last, and only if absolutely necessary.

DKP provides fine-grained control over which pods and nodes are subject to eviction:

- `spec.podLabelSelector` — limits evicted pods by label;
- `spec.namespaceLabelSelector` — filters namespaces whose pods can be considered for eviction;
- `spec.nodeLabelSelector` — selects target nodes by label.

Each of these fields supports standard `matchExpressions` and `matchLabels` syntax, allowing you to use operators like `In`, `NotIn`, `Exists`, and `DoesNotExist` with desired label values.

### How to enable or disable pod eviction

To enable the pod redistribution feature, you need to enable the [`descheduler`](/modules/descheduler/) module.

You can do this in one of the following ways:

1. Using a ModuleConfig resource (e.g., `ModuleConfig/descheduler`). Set the `spec.enabled` field to `true` or `false`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: descheduler
   spec:
     enabled: true    # or false to disable
   ```

1. Using the `d8` command (in the `d8-system/deckhouse` pod):

   ```console
   d8 system module enable descheduler
   # or to disable:
   d8 system module disable descheduler
   ```

1. Through the [Deckhouse web interface](/modules/console/):

   - Go to the “Deckhouse → Modules” section;
   - Find the `descheduler` module and click on it;
   - Toggle the “Module enabled” switch.

The module does not require mandatory configuration. You can enable it without additional settings — it will work with the default values.

### Descheduling strategies

The [`spec.strategies`](/modules/descheduler/cr.html#descheduler-v1alpha2-spec-strategies) parameter lists the strategies you want to enable or configure. Each strategy has an `enabled` flag (default is `false`).

Below is a list of the main strategies available in DKP.

**HighNodeUtilization** — concentrates workloads on fewer nodes by evicting pods from underutilized nodes so they can be rescheduled elsewhere. Requirements:

- Specific `descheduler` module configuration — `MostAllocated`;
- (Optional) cluster autoscaling must be enabled — to allow unused nodes to be shut down.

This strategy is enabled using the `spec.strategies.highNodeUtilization.enabled` parameter.

The `thresholds` parameter defines the resource usage levels below which a node is considered underutilized. If usage (CPU, memory, etc.) falls below *all* threshold values, the node is deemed underutilized, and DKP will attempt to evict pods from it.

Example:

```yaml
---
apiVersion: deckhouse.io/v1alpha2
kind: Descheduler
metadata:
  name: high-node-utilization
spec:
  strategies:
    highNodeUtilization:
      enabled: true
      thresholds:
        cpu: 50
        memory: 50
```

{% alert level="info" %}
In GKE (Google Kubernetes Engine), it is not possible to configure `MostAllocated` by default, but you can use the `optimize-utilization` strategy instead.
{% endalert %}

**LowNodeUtilization** — more evenly distributes workloads across nodes. This strategy identifies underutilized nodes and evicts pods from overutilized ones, assuming that evicted pods will be rescheduled on the underutilized nodes.

Enable the strategy via `spec.strategies.lowNodeUtilization.enabled`.

- An **underutilized** node is one where all resources are below the thresholds set in `strategies.lowNodeUtilization.thresholds`.
- An **overutilized** node exceeds at least one of the values in `strategies.lowNodeUtilization.targetThresholds`.

Nodes with resource usage between `thresholds` and `targetThresholds` are considered optimally utilized and will not have pods evicted.

Example:

```yaml
---
apiVersion: deckhouse.io/v1alpha2
kind: Descheduler
metadata:
  name: low-node-utilization
spec:
  strategies:
    lowNodeUtilization:
      enabled: true
      thresholds:
        cpu: 20
      targetThresholds:
        cpu: 50
```

**RemoveDuplicates** — prevents multiple pods from the same controller (ReplicaSet/ReplicationController/StatefulSet/Job) from running on the same node simultaneously (DaemonSets are excluded). If, for any reason, 2 or more pods from the same controller end up on a single node, this strategy will evict the "extra" pods to redistribute them more evenly across the cluster.

This situation can occur after temporary node failures — when pods are rescheduled to other nodes, and once the original node is back online, the pods remain concentrated on the fallback node.

Enable the strategy using the `strategies.removeDuplicates.enabled` parameter.

Example:

```yaml
spec:
  strategies:
    removeDuplicates:
      enabled: true
```

**RemovePodsViolatingInterPodAntiAffinity** — evicts any pods that violate [`inter-pod affinity/anti-affinity`](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/) rules. For example, if `pod2` and `pod3` have an `anti-affinity` rule against `pod1`, but all three end up on the same node, this strategy will evict `pod1` so that `pod2` and `pod3` can continue operating according to their scheduling constraints.

Enable the strategy using the `strategies.removePodsViolatingInterPodAntiAffinity.enabled` parameter.

Example:

```yaml
spec:
  strategies:
    removePodsViolatingInterPodAntiAffinity:
      enabled: true
```

**RemovePodsViolatingNodeAffinity** — this strategy evicts pods that no longer comply with their defined [`node affinity`](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity) rules. `Node affinity` defines which nodes are suitable for a pod at scheduling time:

1. `requiredDuringSchedulingIgnoredDuringExecution`:
   - A pod with this rule must be scheduled only on a node that satisfies the specified conditions (e.g., a specific label).
   - If the node later no longer satisfies the condition (e.g., the label is removed), and there is another suitable node in the cluster, DKP will evict the pod so it can be rescheduled on a valid node.

1. `preferredDuringSchedulingIgnoredDuringExecution`:
   - A pod with this preference may run on a node that doesn't fully match the affinity, if no better option is available.
   - If a more suitable node appears later in the cluster, DKP may evict the pod so it can be restarted under better placement conditions.

This strategy helps ensure pods continue to align with their node affinity rules and do not remain in suboptimal locations when better options become available.

Enable the strategy using the `spec.strategies.removePodsViolatingNodeAffinity.enabled` parameter.

Example:

```yaml
spec:
  strategies:
    removePodsViolatingNodeAffinity:
      enabled: true
      nodeAffinityType:
        - requiredDuringSchedulingIgnoredDuringExecution
        - preferredDuringSchedulingIgnoredDuringExecution
```
