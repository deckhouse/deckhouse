---
title: "The vertical-pod-autoscaler module: custom resources"
---

## VerticalPodAutoscaler

- `spec.targetRef`:
  - `apiVersion` — API version of the object;
  - `kind` — object type;
  - `name` — object name.
- (optional) `spec.updatePolicy.updateMode`: `Auto`, `Recreate`, `Initial`, `Off` (default is `Auto`)
- (optional) `resourcePolicy.containerPolicies` for specific containers:
    - `containerName` — container name;
    - `mode` — `Auto` or `Off`, to enable or disable autoscaling for the container;
    - `minAllowed` — the minimum amount of `cpu` and `memory` resources for the container;
    - `maxAllowed` — the maximum amount of `cpu` and `memory` resources for the container.
