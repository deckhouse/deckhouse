---
title: "The descheduler module: examples"
---

## Example LowNodeUtilization strategy

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: Descheduler
metadata:
  name: lownodeutilization
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
apiVersion: deckhouse.io/v1alpha1
kind: Descheduler
metadata:
  name: highnodeutilization
spec:
  strategies:
    highNodeUtilization:
      thresholds:
        cpu: 50
        memory: 50
```
