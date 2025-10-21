---
title: "VPA"
permalink: en/architecture/vpa.html
---

## VPA limitations

Before using the vertical pod autoscaler (VPA), you need to consider several limitations:

- Pod restarts when resources change:
  - Updating requested resources is an experimental feature. Each time VPA changes the resources, it recreates the pod, which may then be scheduled on a different node.
  - Pods can be rescheduled on other nodes.

- Compatibility with HPA:
  - It is not recommended to use VPA together with HPA based on CPU and memory.
  - VPA can be used with HPA based on custom or external metrics.

- Issues in large clusters: VPA can work in large clusters, but the load on VPA increases as the number of pods grows.

- Issues with Pending pods: VPA may recommend resources that exceed those available in the cluster, causing pods to get stuck in the `Pending` state.

- Problems when deleting VPA: If you delete or disable VPA (Off mode), the resources will remain at the last modified values. This can cause confusion when resource values differ between the Helm charts, the controller, and the pods themselves.

- Using multiple VPA resources for a single pod: This can lead to unpredictable behavior.

{% alert level="warning" %}
When using VPA, it is recommended to configure a [Pod Disruption Budget](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/).
{% endalert %}

VPA consists of three components:

- Recommender: Monitors current resource consumption (using the Metrics API implemented by the prometheus-metrics-adapter module) and historical resource consumption (querying Trickster before Prometheus) to provide CPU and memory recommendations for containers.
- Updater: Verifies that the pods with VPA have the correct resource requests. If not, it terminates these pods so that the controller recreates them with the new requested resources.
- Admission Plugin: Sets the requested resources when creating new pods (either by the controller or triggered by the Updater).

When the Updater changes resources, it uses the Eviction API, so the Pod Disruption Budget is respected for the updated pods.

![VPA Architecture](../images/vpa/vpa-architecture-en.png)
