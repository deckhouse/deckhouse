---
title: "The vertical-pod-autoscaler module"
search: autoscaler
description: "Automatic calculation and setting of resource request parameters for pods in the Deckhouse Kubernetes Platform cluster."
---

Vertical Pod Autoscaler ([VPA](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler)) is an infrastructure service that allows you to avoid setting exact resource requests for a container if their values are not known in advance. VPA can automatically adjust the CPU and memory reservations (providing that the corresponding mode is enabled) based on the actual resource consumption (as shown by the Prometheus data).
Also, VPA can recommend values for resource requests and limits without updating them automatically.

VPA has the following operating modes:

- `"InPlaceOrRecreate"` (default) — available in Kubernetes 1.33 and later. VPA attempts to update Pod resources without recreating the Pod. If in-place resource updates are not possible, VPA recreates the Pod similarly to the `Recreate` mode.
- `"Auto"` — a deprecated mode. It will no longer be supported in future Deckhouse Kubernetes Platform (DKP) versions. Use an explicit mode instead, such as `InPlaceOrRecreate`, `Recreate`, or `Initial`.
- `"Recreate"` — the mode allows VPA to change resources of running pods (restart them while running). In case of one pod running (`replicas: 1`), this will lead to the service being unavailable during the restart. In this mode, VPA does not recreate pods that were created without a controller.
- `"Initial"` — VPA modifies Pod resources only when Pods are started (but not during operation).
- `"Off"` — VPA does not take any action to update the resource requests for the running containers. In this case, if VPA is running in this mode, you can see what resource values it recommends (`d8 k describe vpa <vpa-name>`).

VPA limitations:

- When using modes that allow Pod recreation, VPA may recreate a Pod if its `resource requests` cannot be updated in place. The new Pod can be scheduled to another node.
- VPA **should not be used concurrently with CPU and memory-based [HPA](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)**. However, you can use VPA together with HPA for custom/external metrics.
- VPA notices almost all `out-of-memory` events, but that does not guarantee its response.
- VPA performance has not been tested for huge clusters.
- VPA recommendations may exceed the available resources in the cluster. That can lead to **Pods becoming Pending**.
- Using multiple VPAs for the same Pod can lead to undefined behavior.
- If VPA is deleted or "turned off" (the `Off` mode), the changes made by VPA earlier are not reset (the most recent value set is kept). It may lead to confusion due to the difference between resource values in Helm/controller and the actual resources of Pods (it may be perceived as if they "came from out of nowhere").

{% alert level="info" %}
We highly recommend using [Pod Disruption Budget](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) with VPA.
{% endalert %}

## Grafana dashboard

Displayed on dashboards:

- `Main / Namespace`, `Main / Namespace / Controller`, `Main / Namespace / Controller / Pod` — the `VPA type` column shows the value of `updatePolicy.updateMode`;
- `Main / Namespaces` — the `VPA %` column shows the percentage of VPA-enabled Pods.

## Vertical Pod Autoscaler Architecture

VPA consists of 3 components:

- `Recommender` — this component monitors the current resource consumption (by making requests to the [Metrics API](https://github.com/kubernetes/design-proposals-archive/blob/main/instrumentation/resource-metrics-api.md) implemented in the [`prometheus-metrics-adapter`](../../modules/prometheus-metrics-adapter/) module) as well as consumption history (by making requests to Trickster caching proxy). As its name suggests, the component provides CPU and memory recommendations for containers.
- `Updater` — this component checks if the Pods have correct resources set and, if not, kills them so that they can be recreated by their controllers with the updated resource requests.
- `Admission Plugin` — this component sets the correct resource requests on new Pods (either just created or recreated by their controller due to Updater's activity).

When the Updater component changes resource values, Pods are evicted using the [Eviction API](https://kubernetes.io/docs/tasks/administer-cluster/safely-drain-node/#the-eviction-api). Thus, the `Pod Disruption Budget` is taken into account for the Pods being updated.

![The VPA Architecture](https://raw.githubusercontent.com/kubernetes/design-proposals-archive/acc25e14ca83dfda4f66d8cb1f1b491f26e78ffe/autoscaling/images/vpa-architecture.png)
