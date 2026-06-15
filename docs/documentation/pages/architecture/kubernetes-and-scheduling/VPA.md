---
title: "Vertical Pod Autoscaler"
permalink: en/architecture/kubernetes-and-scheduling/vpa.html
search: autoscaler architecture, vertical scaling, resource optimization, pod scaling, vpa, vertical pod autoscaler, vertical-pod-autoscaler
description: VPA operating modes and limitations in Deckhouse Kubernetes Platform.
relatedLinks:
  - title: "Enabling vertical scaling"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/vpa.html
---

The [`vertical-pod-autoscaler`](/modules/vertical-pod-autoscaler/) module provides [Vertical Pod Autoscaler (VPA)](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler) in Deckhouse Kubernetes Platform (DKP).

For details on module configuration and usage examples, refer to the [relevant documentation section](/modules/vertical-pod-autoscaler/configuration.html).

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

When VPA is enabled and configured, resource requests are set automatically based on Prometheus data. You can also configure the system to only provide recommendations without applying any changes. For details on enabling and configuring the VPA, refer to [Administration](../../admin/configuration/app-scaling/vpa.html).

## VPA limitations

Before using the vertical pod autoscaler (VPA), you need to consider several limitations:

- Pod restarts when resources change:
  - Updating requested resources is an experimental feature. Each time VPA changes the resources, it recreates the pod, which may then be scheduled on a different node.
  - Pods can be rescheduled on other nodes.

- Compatibility with [Horizontal Pod Autoscaler (HPA)](../../admin/configuration/app-scaling/hpa.html):
  - It is not recommended to use VPA together with HPA that is set for scaling based on CPU and memory.
  - VPA can be used with HPA that is set for scaling based on custom or external metrics.

- Issues in large clusters: VPA can work in large clusters, but the load on VPA increases as the number of pods grows.

- Issues with Pending pods: VPA may recommend resources that exceed those available in the cluster, causing pods to get stuck in the `Pending` state.

- Problems when deleting VPA: If you delete or disable VPA (Off mode), the resources will remain at the last modified values. This can cause confusion when resource values differ between the Helm charts, the controller, and the pods themselves.

- Using multiple VPA resources for a single pod: This can lead to unpredictable behavior.

{% alert level="warning" %}
When using VPA, it is recommended to configure a [Pod Disruption Budget](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/).
{% endalert %}

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`vertical-pod-autoscaler`](/modules/vertical-pod-autoscaler/) module and its interactions with other DKP components are shown in the following diagram:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_EN --->
![vertical-pod-autoscaler module architecture](../../images/architecture/kubernetes-and-scheduling/c4-l2-vertical-pod-autoscaler.png)

## Module components

The `vertical-pod-autoscaler` module consists of the following components:

1. **Vpa-admission-controller** (Deployment): A [VPA](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler) controller that handles the [VerticalPodAutoscaler](/modules/vertical-pod-autoscaler/cr.html#verticalpodautoscaler) custom resource.

   The vpa-admission-controller component performs the following actions:

   * Validates VerticalPodAutoscaler custom resources.
   * When a Pod is created and the VPA mode is not [Off](./vpa.html#vpa-operating-modes), the controller automatically sets or updates `requests` and `limits` in containers, optimizing them according to the current recommendations. The controller updates `limits` values only if the [`spec.resourcePolicy.containerPolicies.controlledValues`](/modules/vertical-pod-autoscaler/cr.html#verticalpodautoscaler-v1-spec-resourcepolicy-containerpolicies-controlledvalues) parameter in the resource management policy is set to `RequestsAndLimits`.

   It consists of the following containers:

   * **admission-controller**: Main container.
   * **kube-rbac-proxy**: Sidecar container with a Kubernetes RBAC-based authorization proxy that provides secure access to admission-controller metrics. It is an [open source project](https://github.com/brancz/kube-rbac-proxy).

1. **Vpa-updater** (Deployment): A [VPA](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler) component that checks whether pods with VPA have the correct resource settings. Vpa-updater performs in-place resource updates through the `pods/resize` Kubernetes subresource; if that is not possible or does not fit the resource management policy, it evicts the Pod.

   It consists of the following containers:

   * **updater**: Main container.
   * **kube-rbac-proxy**: Sidecar container with a Kubernetes RBAC-based authorization proxy that provides secure access to updater metrics. It is an [open source project](https://github.com/brancz/kube-rbac-proxy).

1. **Vpa-recommender** (Deployment): A [VPA](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler) component that calculates recommendations for `requests` based on past and current pod resource consumption.

   Vpa-admission-controller and vpa-updater recalculate `limits` values proportionally to `requests` values if the [`spec.resourcePolicy.containerPolicies.controlledValues`](/modules/vertical-pod-autoscaler/cr.html#verticalpodautoscaler-v1-spec-resourcepolicy-containerpolicies-controlledvalues) parameter in resource management policy is set to `RequestsAndLimits`.

   It consists of the following containers:

   * **recommender**: Main container.
   * **kube-rbac-proxy**: Sidecar container with a Kubernetes RBAC-based authorization proxy that provides secure access to recommender metrics. It is an [open source project](https://github.com/brancz/kube-rbac-proxy).

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   * Watches standard resources such as ConfigMap, Node, LimitRange, and Pod, as well as VerticalPodAutoscaler and VerticalPodAutoscalerCheckpoint custom resources.
   * Retrieves current resource consumption through the [Metrics API](https://github.com/kubernetes/design-proposals-archive/blob/main/instrumentation/resource-metrics-api.md).
   * Evicts running pods when their resource specifications do not match the recommended values.
   * Authorizes requests for metrics.

1. **Prometheus**: Retrieves the history of pod resource consumption metrics.

The following external components interact with the module:

1. **Kube-apiserver**:

   * Validates VerticalPodAutoscaler custom resources.
   * Changes `requests` and `limits` in the Pod specification.

1. **Prometheus**: Collects module metrics.
