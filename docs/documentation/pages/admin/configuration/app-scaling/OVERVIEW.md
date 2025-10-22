---
title: "Overview"
permalink: en/admin/configuration/app-scaling/overview.html
description: "Configure application scaling and pod management in Deckhouse Kubernetes Platform. HPA, VPA, pod eviction, and resource optimization for high availability and efficient resource utilization."
---

Scaling applications and managing pod placement in Deckhouse Kubernetes Platform allows dynamic adaptation of cluster resources to the current load, ensuring high availability of services and efficient resource utilization.

Deckhouse supports all major Kubernetes scaling and workload distribution mechanisms:

- [Horizontal Pod Autoscaling (HPA)](./hpa.html) — automatic adjustment of the number of pod replicas based on resource consumption metrics or external indicators.
- [Vertical Pod Autoscaling (VPA)](./vpa.html) — automatic tuning of requested CPU and memory resources for containers based on actual usage.
- [Scaling by metrics](./scaling-by-metrics.html) — using arbitrary metrics to flexibly manage application scaling through Prometheus.
- [Pod redistribution (Descheduler)](./pod-eviction/descheduler.html#pod-redistribution) — automatically evicting pods to optimize workload placement across the cluster.
- [Pod priorities (Priority Classes)](./pod-eviction/priority-classes.html) — managing the eviction order of pods during resource shortages based on their importance.
- [Scheduler](./pod-eviction/scheduler.html) — configuring the rules and logic for node selection when placing pods.

Deckhouse Kubernetes Platform enables automatic application scaling and efficient resource management to ensure stable and predictable cluster operation.

The following sections provide a detailed description of scaling capabilities, configuration examples, and recommendations for their effective use.
