---
title: "The descheduler module: examples"
---

## Example LowNodeUtilization strategy

```yaml
---
apiVersion: deckhouse.io/v1alpha2
kind: Descheduler
metadata:
  name: low-node-utilization
spec:
  strategies:
    lowNodeUtilization:
      thresholds:
        cpu: 20
      targetThresholds:
        cpu: 50
```

## Example HighNodeUtilization strategy

```yaml
---
apiVersion: deckhouse.io/v1alpha2
kind: Descheduler
metadata:
  name: high-node-utilization
spec:
  strategies:
    highNodeUtilization:
      thresholds:
        cpu: 50
        memory: 50
```
