---
title: "Priority classes"
permalink: en/admin/configuration/app-scaling/pod-eviction/priority-classes.html
description: "Configure pod priority classes in Deckhouse Kubernetes Platform. Pod eviction policies, resource allocation priorities, and cluster resource management optimization."
---

Deckhouse Kubernetes Platform (DKP) creates a set of priority classes (PriorityClass) in the cluster and assigns them to its components. Applications can use these classes by setting the `priorityClassName` field. The scheduler takes pod priority into account: if resources are insufficient, pods with lower priority are preempted first.

For example, if a pod with the `production-low` class cannot be scheduled due to insufficient resources, the scheduler first preempts pods with a lower priority, such as `develop`, then `cluster-low`, and so on. If `priorityClassName` is not set, the pod is treated as having the lowest priority.

## Available priority classes

The table lists the priority classes that DKP creates in the cluster. Choose a class based on the environment and workload criticality.

{% alert level="danger" %}
Do not use the `system-node-critical`, `system-cluster-critical`, `cluster-medium`, or `cluster-low` priority classes, as they are reserved for critical cluster components.
{% endalert %}

| Priority class             | Description                                                                                                                                                                                                                  | Value        |
|----------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------|
| `system-node-critical`     | Cluster components that must be present on the node. Also fully protected from eviction by the kubelet. <br/> Examples: `node-exporter`, `csi`, and others.                                                                  | 2000001000   |
| `system-cluster-critical`  | Cluster components without which the cluster cannot operate correctly. MutatingWebhooks and Extension API servers must use this priority class. Also fully protected from eviction by the kubelet. <br/> Examples: `kube-dns`, `kube-proxy`, `cni-flannel`, `cni-cilium`, and others. | 2000000000   |
| `production-high`          | Stateful applications whose absence in a production environment leads to complete service unavailability or data loss. <br/> Examples: `PostgreSQL`, `Memcached`, `Redis`, `MongoDB`, and others.                            | 9000         |
| `cluster-medium`           | Cluster components that affect monitoring (alerts, diagnostics) and autoscaling. Without monitoring, you cannot assess the scope of an incident; without autoscaling, applications cannot get the resources they need. <br/> Examples: `deckhouse`, `node-local-dns`, `grafana`, `upmeter`, and others. | 7000         |
| `production-medium`        | Main stateless applications in production that serve end users.                                                                                                                        | 6000         |
| `deployment-machinery`     | Cluster components used for building and deploying workloads to the cluster.                                                                                                                                                  | 5000         |
| `production-low`           | Production applications (cron jobs, admin panels, batch processes) that can be unavailable for some time. If batch or cron jobs must not be interrupted, assign them `production-medium`.                                      | 4000         |
| `staging`                  | Staging environments for applications.                                                                                                                                                                                         | 3000         |
| `cluster-low`              | Cluster components that are not required for operation but are desirable. <br/> Examples: `dashboard`, `cert-manager`, `prometheus`, and others.                                                                               | 2000         |
| `develop` (default)        | Development environments for applications. Default class if no other class is specified.                                                                                                                                     | 1000         |
| `standby`                  | Not intended for applications. Used for system purposes to reserve nodes.                                                                                                                                                    | -1           |

## Creating additional priority classes

In addition to the classes that DKP creates in the cluster, you can create an additional priority class by specifying a name and a numeric priority value. Assign the class to pods using the `priorityClassName` field.

Create a file `critical-applications.yaml` with the following content — a `critical-applications` priority class with priority `8000`:

```yaml
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: critical-applications
value: 8000
globalDefault: false
description: "Priority for critical applications"
```

A PriorityClass resource manifest uses the following fields:

- `metadata.name` — the priority class name referenced by the `priorityClassName` field in pod specifications.
- `value` — numeric priority. The higher the value, the higher the priority.
- `globalDefault` — specifies whether this class is assigned to pods that do not explicitly set `priorityClassName`.
- `description` — class description.

{% alert level="warning" %}
Be careful with the `globalDefault: true` field. If you enable it for an additional class, that class will be assigned to pods that do not explicitly specify `priorityClassName`, which may cause unexpected preemption of system components.
{% endalert %}

Apply the manifest to the cluster:

```shell
d8 k apply -f critical-applications.yaml
```

Verify that the resource was created:

```shell
d8 k get priorityclass critical-applications
```

Example output:

```console
NAME              VALUE   GLOBAL-DEFAULT   AGE   PREEMPTIONPOLICY
critical-applications   8000    false            7s    PreemptLowerPriority
```

{% alert level="info" %}
Do not create additional classes with values above 1,000,000 to avoid disrupting critical system components.
{% endalert %}

## Preemption mechanism

### How it works

The scheduler makes preemption decisions based on numeric priority values. The workload type (Deployment, StatefulSet, DaemonSet, or standalone pod) does not affect preemption decisions. The scheduler compares pod priority values regardless of the workload type: a higher-priority pod can preempt a lower-priority pod to free resources on the node.

Preemption does not occur when priorities are equal. If a new pod has the same priority as existing pods, the scheduler does not preempt them. The new pod remains `Pending` until sufficient resources become available.

For practical scenarios and step-by-step demonstrations, see [Using priority classes](/products/kubernetes-platform/documentation/v1/user/configuration/app-scaling/priority-classes.html).

### Protection from eviction

The only way to protect a pod from preemption is to assign it a sufficiently high priority. However, even with a high priority, the pod will be preempted if another pod with an even higher priority appears in the cluster.

To reduce the risks associated with preemption, combine the following three mechanisms. These mechanisms are universal and apply to any critical applications:

1. A high `priorityClassName` — reduces the likelihood that the pod will be selected for preemption.
1. PodDisruptionBudget (PDB) — limits how many replicas can be disrupted at the same time. During preemption, the scheduler treats PDB constraints as best-effort rather than strict requirements (unlike planned operations such as draining a node, where PDB is a hard constraint).
1. `terminationGracePeriodSeconds` — gives the application time to flush buffers and close connections before forced termination. This helps preserve data integrity when a pod must be terminated, including cases where the PDB cannot be honored.

{% alert level="info" %}
Without the `terminationGracePeriodSeconds` parameter on a pod, an immediate termination may lead to loss of data that was not written to disk and, for example, to database corruption.
{% endalert %}

### Differences from other resource management mechanisms

Priority classes serve a different purpose from other resource management mechanisms:

| Mechanism | Purpose | Difference from priority classes |
|-----------|---------|----------------------------------|
| ResourceQuota | Limits resource consumption in a namespace | Limits total resource consumption within a namespace; does not determine preemption order. |
| LimitRange | Default limits and requests for pods | Sets minimum and maximum values; does not affect preemption. |

Unlike ResourceQuota and LimitRange, priority classes determine which pods take precedence when resources are insufficient; they do not limit resource consumption.

## Operations and diagnostics

If a pod does not start despite available resources in the cluster, the issue is often related to pod placement rather than its priority class.

Possible cluster-level causes:

- Available CPU and memory are distributed across multiple nodes, and no single node has enough resources to accommodate the pod, even after preemption.
- No eligible pods can be preempted because all pods on the target node have equal or higher priority. How to check — see the [Practical check when no suitable pods are available](/products/kubernetes-platform/documentation/v1/user/configuration/app-scaling/priority-classes.html#practical-check-when-no-suitable-pods-are-available) user section.
- No nodes satisfy the pod's scheduling requirements, for example because of taints that the pod does not tolerate.
- The pod limit per node (`Capacity.pods`) has been reached. Preemption cannot resolve this condition because replacing one pod with another does not reduce the total pod count.

What you can do:

1. Add worker nodes or redistribute workloads if available resources are spread across different nodes.
1. Free resources on the overloaded node by removing or relocating less critical pods.
1. Review the density of DaemonSet and system pods on the node.
1. If needed, assign the application a higher priority class [from Available priority classes](#available-priority-classes).
