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

## Example LowNodeUtilization strategy with thresholds relative to the average

The strategy keeps the CPU utilization of the nodes within the corridor of ±5% around the average utilization of the cluster nodes and evicts at most two pods from one node per descheduling cycle. A virtual machine that is still migrating from the node takes one of the two places:

```yaml
---
apiVersion: deckhouse.io/v1alpha2
kind: Descheduler
metadata:
  name: low-node-utilization-deviation
spec:
  strategies:
    lowNodeUtilization:
      enabled: true
      useDeviationThresholds: true
      thresholds:
        cpu: 5
      targetThresholds:
        cpu: 5
      evictionLimits:
        node: 2
```

## Example LowNodeUtilization strategy on the 15-minute CPU average

The strategy keeps the nodes within the corridor of ±5% around the cluster average of the CPU utilization averaged over 15 minutes, so short load spikes do not trigger evictions. It evicts one pod from each over utilized node per descheduling cycle; keep `evictionLimits.node` unset in this mode, since it raises that number. Use the default `Moderate` descheduling interval (15 minutes). The `prometheus` module must be enabled:

```yaml
---
apiVersion: deckhouse.io/v1alpha2
kind: Descheduler
metadata:
  name: low-node-utilization-prometheus
spec:
  strategies:
    lowNodeUtilization:
      enabled: true
      metricsSource: Prometheus
      useDeviationThresholds: true
      thresholds:
        cpu: 5
      targetThresholds:
        cpu: 5
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

## Protecting pods by StorageClass

Use the [`protectedStorageClasses`](cr.html#descheduler-v1alpha2-spec-protectedstorageclasses) parameter to forbid evicting pods that use PersistentVolumeClaims provisioned by specific StorageClasses. This is useful for storage that is bound to a node (for example, `local-path`), where evicting a pod either loses the data locality or leaves the pod unschedulable.

```yaml
---
apiVersion: deckhouse.io/v1alpha2
kind: Descheduler
metadata:
  name: protect-local-storage
spec:
  protectedStorageClasses:
    - local-path
  strategies:
    lowNodeUtilization:
      enabled: true
      thresholds:
        cpu: 20
      targetThresholds:
        cpu: 50
```

With this configuration, the descheduler can still evict every other pod, but a pod with a PersistentVolumeClaim from the `local-path` StorageClass is never evicted.

Keep the following in mind:

- If the parameter is omitted or empty, the behavior does not change: pods with PersistentVolumeClaims are evicted as usual.
- A pod is also considered protected when its PersistentVolumeClaim cannot be resolved, that is, the PVC does not exist or its `storageClassName` is empty. Such pods are never evicted while the parameter is set.
