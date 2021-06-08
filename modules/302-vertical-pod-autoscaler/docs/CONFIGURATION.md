---
title: "The vertical-pod-autoscaler: configuration"
search: autoscaler
---

This module is **enabled** by default in clusters from version 1.11 onward. Generally, no configuration is required.

VPA works directly with the pod (instead of the pod controller) by measuring and changing its containers' parameters. Configuring is performed using the [`VerticalPodAutoscaler`](cr.html#verticalpodautoscaler) custom resource.

## Parameters

The module only has the `nodeSelector/tolerations` settings.

<!-- SCHEMA -->
