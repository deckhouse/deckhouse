---
title: "The descheduler module: examples"
---

## Example CR

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Descheduler
metadata:
  name: example
spec:
  deschedulerPolicy:
    # provide common parameters that apply to all strategies
    globalParameters:
      evictFailedBarePods: true
    strategies:
      # enable a strategy
      podLifeTime:
        enabled: true

      # enable a strategy and set additional parameters
      removeDuplicates:
        enabled: true
        parameters:
          nodeFit: true
```
