---
title: "The vertical-pod-autoscaler: configuration"
search: autoscaler
---

This module is **enabled** by default in clusters from version 1.11 onward. Generally, no configuration is required.

VPA works directly with the Pod (instead of the Pod controller) by measuring and changing its containers' parameters. Configuring is performed using the [`VerticalPodAutoscaler`](cr.html#verticalpodautoscaler) Custom Resource.

## Parameters

The module only has the `nodeSelector/tolerations` settings.

<!-- SCHEMA -->
