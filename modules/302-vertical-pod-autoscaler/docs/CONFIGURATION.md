---
title: "The vertical-pod-autoscaler: configuration"
search: autoscaler
---

This module is **enabled** by default in clusters from version 1.11 onward. Generally, no configuration is required.

## Parameters

The module only has the `nodeSelector/tolerations` settings:
* `nodeSelector` — the same as in the pods' `spec.nodeSelector` parameter in Kubernetes;
    * If the parameter is omitted, it will be set [automatically](../../#advanced-scheduling).
    * You can set it to `false` to avoid adding any nodeSelector.
* `tolerations` — the same as in the pods' `spec.tolerations` parameter in Kubernetes;
    * If the parameter is omitted, it will be set [automatically](../../#advanced-scheduling).
    * You can set it to `false` to avoid adding any tolerations.

### Examples
```yaml
verticalPodAutoscaler: |
  nodeSelector:
    node-role/example: ""
  tolerations:
  - key: dedicated
    operator: Equal
    value: example
```

VPA works directly with the pod (instead of the pod controller) by measuring and changing its containers' parameters. Configuring is performed using the [`VerticalPodAutoscaler`](cr.html#verticalpodautoscaler) custom resource.
