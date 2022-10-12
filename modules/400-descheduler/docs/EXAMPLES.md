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
    parameters:
      evictFailedBarePods: true
    strategies:
      # enable a strategy and specify its parameters
      podLifeTime:
        params:
          podLifeTime:
            maxPodLifeTimeSeconds: 86400
            podStatusPhases:
              - Pending

      # enable a strategy, but let its parameters be defaulted
      removeDuplicates: { }
```
