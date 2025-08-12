---
title: The prometheus-metrics-adapter module
search: autoscaler, HorizontalPodAutoscaler
description: "Ensuring the operation of horizontal and vertical autoscaling based on any metrics in the cluster of the Deckhouse Kubernetes Platform."
---

This module allows [HPA](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/) and [VPA](../../modules/vertical-pod-autoscaler/) autoscalers base their decisions on various metrics.

It installs an [implementation](https://github.com/kubernetes-sigs/prometheus-adapter) of the Kubernetes [resource metrics API](https://github.com/kubernetes/design-proposals-archive/blob/main/instrumentation/resource-metrics-api.md), [custom metrics API](https://github.com/kubernetes/design-proposals-archive/blob/main/instrumentation/custom-metrics-api.md), and [external metrics API](https://github.com/kubernetes/design-proposals-archive/blob/main/instrumentation/external-metrics-api.md) to get Prometheus metrics.

As a result:
- `kubectl top` can collect Prometheus metrics via the adapter;
- [autoscaling/v2](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.23/#objectmetricsource-v2-autoscaling) can be used for scaling applications (HPA);
- Prometheus data can be obtained using the Kubernetes API and utilized in other modules (Vertical Pod Autoscaler, etc.).

The following parameters serve as a basis for scaling:
* CPU (of the Pod),
* memory (of the Pod),
* rps (of the Ingress) — over 1,5,15 minutes (`rps_Nm`),
* CPU (of the Pod) — over 1,5,15 minutes (`cpu_Nm`) — average CPU utilization over N minutes,
* memory (of the Pod) — over 1,5,15 minutes (`memory_Nm`) — average Memory utilization over N minutes,
* any Prometheus metrics and any queries based on them.

## How does it work?

This module registers `k8s-prometheus-adapter` as an external API service that extends the capabilities of the Kubernetes API. When some Kubernetes component (VPA, HPA) needs information about the resources consumed, it requests the Kubernetes API, which, in turn, proxies that request to the adapter. The adapter figures out (using its [configuration file](https://github.com/deckhouse/deckhouse/blob/main/modules/301-prometheus-metrics-adapter/templates/config-map.yaml)) how to calculate the metric and sends a request to Prometheus.
