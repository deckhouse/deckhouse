---
title: "VPA"
permalink: en/architecture/vpa.html
search: autoscaler architecture, vertical scaling, resource optimization, pod scaling
description: VPA operating modes and limitations in Deckhouse Kubernetes Platform.
relatedLinks:
  - title: "Enabling vertical scaling"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/vpa.html
---

## VPA operating modes

VPA can operate in two modes:

- Automatic resource adjustment:

  - **InPlaceOrRecreate** (the default in Kubernetes starting from version 1.33): VPA attempts to update resources without recreating Pods. If in-place resource updates are not possible, VPA falls back to behavior similar to the **Recreate** mode: the Pod for which the resources cannot be updated is evicted, and the controller creates a new Pod with updated resources.

    > To use the **InPlaceOrRecreate** mode in Kubernetes versions earlier than 1.33, enable the `InPlacePodVerticalScaling` feature gate in the [`control-plane-manager` configuration](/modules/control-plane-manager/configuration.html#parameters-enabledfeaturegates).
  
  - **Auto** (the default in Kubernetes versions earlier than 1.33): VPA changes resource requests without recreating Pods but behaves the same as **Recreate** and restarts the Pod when necessary. This is a deprecated operating mode that will no longer be supported in future Deckhouse Kubernetes Platform (DKP) versions.

  - **Recreate**: VPA adjusts the resources of running Pods by restarting them. For a single Pod (`replicas: 1`), this will result in service unavailability during the restart. VPA does not restart Pods that were created without a controller.

- Recommendations only, without modifying resources:

  - **Initial**: Pod resources are adjusted only when Pods are created and not during the runtime.

  - **Off**: VPA does not change resources automatically. However, it still provides resource recommendations, which can be viewed using the `d8 k describe vpa` command.

When VPA is enabled and configured, resource requests are set automatically based on Prometheus data. You can also configure the system to only provide recommendations without applying any changes. For details on enabling and configuring the VPA, refer to [Administration](../admin/configuration/app-scaling/vpa.html).

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
