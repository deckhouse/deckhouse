---
title: "The vertical-pod-autoscaler: configuration"
search: autoscaler
---

VPA works directly with the Pod (instead of the Pod controller) by measuring and changing its containers' parameters. Configuring is performed using the [`VerticalPodAutoscaler`](cr.html#verticalpodautoscaler) Custom Resource.

The module generally requires no configuration and only has the `nodeSelector/tolerations` settings.

<!-- SCHEMA -->
