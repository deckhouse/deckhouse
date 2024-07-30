---
title: "The monitoring-ping module: usage"
---

## Adding extra IP addresses to monitor

To monitor additional IP addresses, use the [externalTargets](configuration.html#parameters-externaltargets) parameters:

Module configuration example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: monitoring-ping
spec:
  version: 1
  enabled: true
  settings:
    externalTargets:
    - name: google-primary
      host: 8.8.8.8
    - name: yaru
      host: ya.ru
    - host: youtube.com
```

> Grafana uses the `name` field to display the related data. If the `name` field is skipped, you must fill in details to the `host` field (mandatory).
