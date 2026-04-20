---
title: "The descheduler module: examples"
---

## Configuring the pod redistribution interval

To set how often the `descheduler` module runs the pod redistribution cycle, use the [`deschedulingInterval`](configuration.html#parameters-deschedulinginterval) parameter.

For example, to run `descheduler` every 5 minutes, set `deschedulingInterval: Frequent`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: descheduler
spec:
  enabled: true
  settings:
    deschedulingInterval: Frequent
```

Supported values:

- `Frequent`: Runs every 5 minutes.
  Suitable for clusters where faster pod redistribution is important.
- `Moderate`: Runs every 15 minutes (default).
- `Rare`: Runs every 30 minutes.
  Suitable for clusters where it's important to minimize the number of pod redistributions.

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
