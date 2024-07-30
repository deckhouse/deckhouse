---
title: "The descheduler module: examples"
---

## Example custom resource

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Descheduler
metadata:
  name: example
spec:
  deschedulerPolicy:
    # Provide common parameters that apply to all strategies.
    globalParameters:
      evictFailedBarePods: true
    strategies:
      # Enable a strategy.
      podLifeTime:
        enabled: true

      # Enable a strategy and set additional parameters.
      removeDuplicates:
        enabled: true
        parameters:
          nodeFit: true
```

## Example custom resource for specific NodeGroup (node labelSelector)

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Descheduler
metadata:
  name: example-specific-ng
spec:
  deploymentTemplate:
    nodeSelector:
      node.deckhouse.io/group: worker
```
