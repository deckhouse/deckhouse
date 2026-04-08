---
title: "The descheduler module: examples"
---

## Configuring the descheduling interval

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: descheduler
spec:
  enabled: true
  settings:
    deschedulingInterval: "Frequent"
```

Available presets:
- `Frequent` — every 5 minutes;
- `Moderate` — every 15 minutes (default);
- `Rare` — every 30 minutes.

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
      enabled: true
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
      enabled: true
      thresholds:
        cpu: 50
        memory: 50
```
