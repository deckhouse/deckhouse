---
title: "The runtime-audit-engine module: usage"
---

## How to collect events

Pods of runtime-audit-engine outputs all events to stdout. 
These events then can be collected by the `log-shipper` module and be sent to any supported destination.

An example of `ClusterLoggingConfig` for `log-shipper`.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: falco-events
spec:
  destinationRefs:
  - xxxx
  kubernetesPods:
    namespaceSelector:
      matchNames:
      - d8-runtime-audit-events
  labelsFilter:
  - operator: Regex
    values: ["\{.*"] # to collect only JSON logs
    field: "message"
  type: KubernetesPods
```

## How to alert

All metrics are automatically collected by Prometheus. Add a CustomPrometheusRule to enable alerts.

```yaml
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  name: falco-critical-alerts
spec:
  groups:
  - name: falco-critical-alerts
    rules:
    - alert: FalcoCriticalAlertsAreFiring
      for: 1m
      annotations:
        description: |
          There is a suspicious activity on a node {{ $labels.node }}. 
          Check you events journal for more details.
        summary: Falco detects a critical security incident
      expr: |
        sum by (node) (rate(falco_events{priority="Critical"}[5m]) > 0)
```
> NOTE: Alerts are best work with an events storage like Elasticsearch or Loki. With an alert users can be notified that something is happening.
> The next step is to find what using previously collected events.
