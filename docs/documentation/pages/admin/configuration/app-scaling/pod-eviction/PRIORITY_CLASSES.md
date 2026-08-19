---
title: "Priority classes"
permalink: en/admin/configuration/app-scaling/pod-eviction/priority-classes.html
description: "Configure pod priority classes in Deckhouse Kubernetes Platform. Pod eviction policies, resource allocation priorities, and cluster resource management optimization."
---

Deckhouse Kubernetes Platform (DKP) provides a set of priority classes (PriorityClass) for its components and for applications running in the cluster. The scheduler takes pod priority into account when distributing workloads: if resources are insufficient, pods with lower priority are preempted first.

For example, if pods have `priorityClassName: production-low` and the cluster lacks resources for them, the scheduler first considers pods with `priorityClassName: develop` for preemption, then pods with `cluster-low`, and so on. When choosing a priority class, consider the application type and the environment it runs in. If a pod does not specify `priorityClassName`, the default class `develop` applies.

## Available priority classes

The table lists available priority classes.

{% alert level="danger" %}
Do not use the `system-node-critical`, `system-cluster-critical`, `cluster-medium`, or `cluster-low` priority classes, as they are reserved for critical cluster components.
{% endalert %}

| Priority class             | Description                                                                                                                                                                                                                  | Value        |
|----------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------|
| `system-node-critical`     | Cluster components that must be present on the node. Also fully protected from eviction by the kubelet. <br/> Examples: `node-exporter`, `csi`, and others.                                                                  | 2000001000   |
| `system-cluster-critical`  | Cluster components without which the cluster cannot operate correctly. MutatingWebhooks and Extension API servers must use this PriorityClass. Also fully protected from eviction by the kubelet. <br/> Examples: `kube-dns`, `kube-proxy`, `cni-flannel`, `cni-cilium`, and others. | 2000000000   |
| `production-high`          | Stateful applications whose absence in a production environment leads to complete service unavailability or data loss. <br/> Examples: `PostgreSQL`, `Memcached`, `Redis`, `MongoDB`, and others.                            | 9000         |
| `cluster-medium`           | Cluster components that affect monitoring (alerts, diagnostics) and autoscaling. Without monitoring, you cannot assess the scope of an incident; without autoscaling, applications cannot get the resources they need. <br/> Examples: `deckhouse`, `node-local-dns`, `grafana`, `upmeter`, and others. | 7000         |
| `production-medium`        | Main stateless applications in production that serve end users.                                                                                                                        | 6000         |
| `deployment-machinery`     | Cluster components used for building and deploying workloads to the cluster.                                                                                                                                                  | 5000         |
| `production-low`           | Production applications (cron jobs, admin panels, batch processes) that can be unavailable for some time. If batch or cron jobs must not be interrupted, assign them `production-medium`.                                      | 4000         |
| `staging`                  | Staging environments for applications.                                                                                                                                                                                         | 3000         |
| `cluster-low`              | Cluster components that are not required for operation but are desirable. <br/> Examples: `dashboard`, `cert-manager`, `prometheus`, and others.                                                                               | 2000         |
| `develop` (default)        | Development environments for applications. Default class if no other class is specified.                                                                                                                                     | 1000         |
| `standby`                  | Not intended for applications. Used for system purposes to reserve nodes.                                                                                                                                                    | -1           |

## Creating custom priority classes

In addition to the classes that DKP creates in the cluster, you can create a custom PriorityClass with a name and a numeric priority value for a specific application. Assign the class to pods using the `priorityClassName` field.

Create a file `my-priority.yaml` with the following content — a PriorityClass named `my-app-critical` with priority `8000`:

```yaml
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: my-app-critical
value: 8000
globalDefault: false
description: "Priority for critical applications"
```

A PriorityClass manifest uses the following fields:

- `metadata.name` — class name. Specify it in pods in the `priorityClassName` field.
- `value` — numeric priority. The higher the value, the higher the priority.
- `globalDefault` — specifies whether this class is assigned to pods that do not explicitly set `priorityClassName`.
- `description` — class description.

{% alert level="warning" %}
Be careful with the `globalDefault: true` field. If you set it for a custom class, this priority will be assigned to all pods that do not explicitly specify a priority class, which may lead to unpredictable preemption of system components.
{% endalert %}

Apply the manifest to the cluster:

```shell
d8 k apply -f my-priority.yaml
```

Verify that the resource was created:

```shell
d8 k get priorityclass my-app-critical
```

Example output:

```console
NAME              VALUE   GLOBAL-DEFAULT   AGE   PREEMPTIONPOLICY
my-app-critical   8000    false            7s    PreemptLowerPriority
```

{% alert level="info" %}
Do not create custom classes with values above 1,000,000 to avoid disrupting critical system components.
{% endalert %}

## Preemption mechanism

### How it works

The scheduler decides on preemption based solely on the numeric priority value. The workload type (Deployment, StatefulSet, DaemonSet, or a standalone pod) does not matter — the scheduler compares only the numeric priority values: a higher-priority pod can preempt a lower-priority pod to free resources for scheduling.

Preemption does not occur when priorities are equal. If a new pod has the same priority as existing pods on the node, the scheduler will not preempt them — the new pod remains in `Pending` status until free resources appear.

For practical scenarios and step-by-step demonstrations, see [Using PriorityClass](/products/kubernetes-platform/documentation/v1/user/configuration/app-scaling/priority-classes.html).

### Protection from eviction

The primary way to reduce the likelihood of a pod being preempted is to assign it a sufficiently high priority. However, even with a high priority, a pod can still be preempted if another pod with an even higher priority appears in the cluster.

To reduce the risks associated with preemption, combine the following mechanisms:

1. A high `priorityClassName` — makes the pod a less likely preemption candidate.
1. PodDisruptionBudget (PDB) — limits the number of replicas removed at the same time. During preemption, PDB acts as a recommendation (unlike planned operations such as `d8 k drain`, where PDB is a hard constraint).
1. `terminationGracePeriodSeconds` — gives the application time to flush buffers and close connections before forced termination. This allows the application to shut down gracefully and helps preserve data integrity even if the PDB cannot be honored.

{% alert level="info" %}
Insufficient time for graceful termination may result in unsaved data being lost, database corruption, or a lengthy filesystem check on the next startup. Graceful shutdown gives the application time to flush pending data and close connections before termination.
{% endalert %}

### Differences from other resource management mechanisms

PriorityClass is often confused with other resource management mechanisms. It is important to understand their differences:

| Mechanism | Purpose | Difference from PriorityClass |
|-----------|---------|-------------------------------|
| ResourceQuota | Limits resource consumption in a namespace | Limits total consumption; does not affect pod preemption. |
| LimitRange | Default limits and requests for pods | Sets minimum and maximum values; does not affect preemption. |

Unlike these mechanisms, PriorityClass affects the order of pod preemption when resources are insufficient, not consumption limits.

## Operations and diagnostics

If pod priority does not work as expected, check scheduler events to determine the cause. This section describes possible causes and cluster-level actions. Practical commands to check specific pods and example output are provided in the [Operations and diagnostics](/products/kubernetes-platform/documentation/v1/user/configuration/app-scaling/priority-classes.html#operations-and-diagnostics) user section.

### Preemption does not occur

If a higher-priority pod remains `Pending` and lower-priority pods are not preempted, check the pod and scheduler events. Typical signs:

- `FailedPreemption` with the message `no preemption victims found for pod`;
- `Preemption is not helpful for scheduling` in `FailedScheduling` events.

Possible causes:

- Resource fragmentation — free CPU and memory are spread across different nodes, and no single node can fit the new pod entirely even after preemption.
- No suitable candidates — all pods on the node have a priority equal to or higher than that of the incoming pod, so the scheduler cannot choose a victim for preemption.

To check this situation for a specific pod, use the commands and examples from the [Practical diagnostics when preemption is impossible](/products/kubernetes-platform/documentation/v1/user/configuration/app-scaling/priority-classes.html#practical-diagnostics-when-preemption-is-impossible) user section.

### No lower-priority pods available

If a pod does not start and scheduler events contain the message `No preemption victims found for incoming pod`, there are no lower-priority pods on the target node that can be preempted.

| Message | Meaning |
|---------|---------|
| `0/N nodes are available` | The cluster has nodes, but none can accommodate the pod. |
| `No preemption victims found for incoming pod` | On the node with insufficient resources, there are no lower-priority pods to preempt. |

Possible cluster-level solutions:

- Assign the application a higher priority class [from Available priority classes](#available-priority-classes).
- Free resources on the node — remove or relocate less critical pods.
- Add nodes or redistribute workloads if the problem is related to resource fragmentation.

To check pod priorities on a specific node and verify that there are no suitable preemption candidates, follow the [Practical check when no suitable pods are available](/products/kubernetes-platform/documentation/v1/user/configuration/app-scaling/priority-classes.html#practical-check-when-no-suitable-pods-are-available) user example.

### Pod limit per node

Even with sufficient CPU and memory, the limit on the maximum number of pods per node (`Capacity.pods`) can prevent a high-priority pod from starting. Preemption cannot resolve this issue because replacing a lower-priority pod with the incoming pod does not reduce the total number of pods on the node.

A typical sign in pod events is `Too many pods`.

Possible cluster-level solutions:

- Add worker nodes and distribute the workload.
- Reduce the number of pods on the overloaded node — remove or relocate low-priority workloads.
- Review the number of DaemonSet and system pods running on the node.

To diagnose a situation where pod startup is limited by the maximum number of pods per node, see the [Practical example "Pod limit per node"](/products/kubernetes-platform/documentation/v1/user/configuration/app-scaling/priority-classes.html#practical-example-pod-limit-per-node) user section.
